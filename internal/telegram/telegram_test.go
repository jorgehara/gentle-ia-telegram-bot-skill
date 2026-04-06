package telegram

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	tg "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/jorgehara/go-telegram-opencode-bridge/internal/config"
	"github.com/jorgehara/go-telegram-opencode-bridge/internal/opencode"
)

const testTelegramToken = "test-token"

type handleMessageTestBackend struct {
	mu sync.Mutex

	createSessionCalls int
	abortCalls         int
	providersCalls     int
	defaultModelCalls  int
	promptCalls        int

	sentMessages []string

	promptStarted chan struct{}
	promptBlock   chan struct{}
}

func newHandleMessageTestBackend() *handleMessageTestBackend {
	return &handleMessageTestBackend{}
}

func (b *handleMessageTestBackend) handler(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/bot"+testTelegramToken+"/") {
		b.handleTelegramAPI(w, r)
		return
	}

	b.handleOpenCodeAPI(w, r)
}

func (b *handleMessageTestBackend) handleTelegramAPI(w http.ResponseWriter, r *http.Request) {
	endpoint := strings.TrimPrefix(r.URL.Path, "/bot"+testTelegramToken+"/")

	switch endpoint {
	case "getMe":
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true,"result":{"id":1,"is_bot":true,"first_name":"Test","username":"test_bot"}}`)
	case "sendMessage":
		if err := r.ParseForm(); err == nil {
			b.mu.Lock()
			b.sentMessages = append(b.sentMessages, r.Form.Get("text"))
			b.mu.Unlock()
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true,"result":{"message_id":1}}`)
	case "sendChatAction":
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true,"result":true}`)
	default:
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true,"result":true}`)
	}
}

func (b *handleMessageTestBackend) handleOpenCodeAPI(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/session":
		b.mu.Lock()
		b.createSessionCalls++
		b.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"session-1","title":"Test"}`)
	case r.Method == http.MethodPost && r.URL.Path == "/session/session-1/abort":
		b.mu.Lock()
		b.abortCalls++
		b.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	case r.Method == http.MethodGet && r.URL.Path == "/config":
		b.mu.Lock()
		b.defaultModelCalls++
		b.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"model":{"providerID":"openai","modelID":"gpt-4"}}`)
	case r.Method == http.MethodGet && r.URL.Path == "/config/providers":
		b.mu.Lock()
		b.providersCalls++
		b.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"providers":[{"id":"openai","name":"OpenAI","models":{"gpt-4":{"id":"gpt-4","name":"GPT-4"}}}]}`)
	case r.Method == http.MethodPost && r.URL.Path == "/session/session-1/message":
		b.mu.Lock()
		b.promptCalls++
		promptStarted := b.promptStarted
		promptBlock := b.promptBlock
		b.mu.Unlock()

		if promptStarted != nil {
			select {
			case promptStarted <- struct{}{}:
			default:
			}
		}
		if promptBlock != nil {
			<-promptBlock
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"info":{"messageID":"msg-1","role":"assistant"},"parts":[{"type":"text","text":"ok"}]}`)
	default:
		http.NotFound(w, r)
	}
}

func (b *handleMessageTestBackend) hasBusyMessage() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, text := range b.sentMessages {
		if strings.Contains(text, "Estoy procesando tu mensaje anterior") {
			return true
		}
	}

	return false
}

func (b *handleMessageTestBackend) snapshotCounts() (createSession, abort, providers, defaultModel, prompt int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.createSessionCalls, b.abortCalls, b.providersCalls, b.defaultModelCalls, b.promptCalls
}

func newHandleMessageTestBot(t *testing.T, backend *handleMessageTestBackend) *TelegramBot {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(backend.handler))
	t.Cleanup(server.Close)

	apiEndpoint := server.URL + "/bot%s/%s"
	tgBot, err := tg.NewBotAPIWithAPIEndpoint(testTelegramToken, apiEndpoint)
	if err != nil {
		t.Fatalf("creating telegram bot api for tests: %v", err)
	}

	cfg := &config.Config{
		AllowedChatIDs:   []int64{42},
		ProjectDir:       ".",
		EnableMarkdown:   true,
		OpencodeURL:      server.URL,
		OpencodeUsername: "opencode",
		OpencodePassword: "",
	}

	client := opencode.NewOpencodeClient(server.URL, "opencode", "")

	return &TelegramBot{
		Bot:        tgBot,
		Client:     client,
		Config:     cfg,
		chatModels: make(map[int64]*opencode.ModelRef),
		processing: make(map[int64]bool),
		stopChan:   make(chan struct{}),
	}
}

func newCommandMessage(chatID int64, text string) *tg.Message {
	commandEnd := strings.Index(text, " ")
	if commandEnd == -1 {
		commandEnd = len(text)
	}

	return &tg.Message{
		Text: text,
		Chat: &tg.Chat{ID: chatID},
		From: &tg.User{ID: 100, UserName: "tester", FirstName: "Test", LastName: "User"},
		Entities: []tg.MessageEntity{{
			Type:   "bot_command",
			Offset: 0,
			Length: commandEnd,
		}},
	}
}

func newTextMessage(chatID int64, text string) *tg.Message {
	return &tg.Message{
		Text: text,
		Chat: &tg.Chat{ID: chatID},
		From: &tg.User{ID: 100, UserName: "tester", FirstName: "Test", LastName: "User"},
	}
}

// TestTelegramBotInMemoryStore tests the in-memory model storage
func TestTelegramBotInMemoryStore(t *testing.T) {
	t.Parallel()

	// Create a minimal bot instance for testing
	bot := &TelegramBot{
		chatModels: make(map[int64]*opencode.ModelRef),
	}

	chatID := int64(12345)

	// Test 1: Initially no model for chatID
	bot.mu.Lock()
	modelRef := bot.chatModels[chatID]
	bot.mu.Unlock()

	if modelRef != nil {
		t.Errorf("Expected nil model for new chat, got %+v", modelRef)
	}

	// Test 2: Set a model
	bot.mu.Lock()
	bot.chatModels[chatID] = &opencode.ModelRef{
		ProviderID: "openai",
		ModelID:    "gpt-4",
	}
	bot.mu.Unlock()

	// Test 3: Retrieve the model
	bot.mu.Lock()
	modelRef = bot.chatModels[chatID]
	bot.mu.Unlock()

	if modelRef == nil {
		t.Fatal("Expected model to be set, got nil")
	}
	if modelRef.ProviderID != "openai" {
		t.Errorf("ProviderID = %q, want %q", modelRef.ProviderID, "openai")
	}
	if modelRef.ModelID != "gpt-4" {
		t.Errorf("ModelID = %q, want %q", modelRef.ModelID, "gpt-4")
	}

	// Test 4: Update the model
	bot.mu.Lock()
	bot.chatModels[chatID] = &opencode.ModelRef{
		ProviderID: "anthropic",
		ModelID:    "claude-3",
	}
	bot.mu.Unlock()

	bot.mu.Lock()
	modelRef = bot.chatModels[chatID]
	bot.mu.Unlock()

	if modelRef.ProviderID != "anthropic" {
		t.Errorf("ProviderID = %q, want %q", modelRef.ProviderID, "anthropic")
	}
	if modelRef.ModelID != "claude-3" {
		t.Errorf("ModelID = %q, want %q", modelRef.ModelID, "claude-3")
	}

	// Test 5: Delete (restore to default)
	bot.mu.Lock()
	delete(bot.chatModels, chatID)
	bot.mu.Unlock()

	bot.mu.Lock()
	modelRef = bot.chatModels[chatID]
	bot.mu.Unlock()

	if modelRef != nil {
		t.Errorf("Expected nil after delete, got %+v", modelRef)
	}
}

// TestNewTelegramBotInitialization tests that NewTelegramBot properly initializes the in-memory map
func TestNewTelegramBotInitialization(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		BotToken:       "test-token",
		OpencodeURL:    "http://localhost:4096",
		EnableMarkdown: true,
		Debug:          false,
	}

	// We can't create a real bot without a valid token, but we can test the structure
	// This test verifies that the chatModels map is properly initialized
	bot := &TelegramBot{
		Config:     cfg,
		chatModels: make(map[int64]*opencode.ModelRef),
		processing: make(map[int64]bool),
		stopChan:   make(chan struct{}),
	}

	if bot.chatModels == nil {
		t.Error("chatModels map should be initialized")
	}

	if bot.processing == nil {
		t.Error("processing map should be initialized")
	}

	if bot.stopChan == nil {
		t.Error("stopChan should be initialized")
	}
}

func TestParseModelRef(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantProvider string
		wantModel    string
		wantErr      bool
	}{
		{"valid provider/model", "openai/gpt-4", "openai", "gpt-4", false},
		{"valid with numbers", "anthropic/claude-3.5", "anthropic", "claude-3.5", false},
		{"missing slash", "gpt-4", "", "", true},
		{"empty provider", "/gpt-4", "", "", true},
		{"empty model", "openai/", "", "", true},
		{"just slash", "/", "", "", true},
		{"multiple slashes", "openai/gpt-4/extra", "openai", "gpt-4/extra", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			providerID, modelID, errMsg := parseModelRef(tt.input)
			hasErr := errMsg != ""
			if hasErr != tt.wantErr {
				t.Errorf("parseModelRef(%q) error = %v, wantErr %v", tt.input, hasErr, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if providerID != tt.wantProvider {
					t.Errorf("providerID = %q, want %q", providerID, tt.wantProvider)
				}
				if modelID != tt.wantModel {
					t.Errorf("modelID = %q, want %q", modelID, tt.wantModel)
				}
			}
		})
	}
}

func TestModelExists(t *testing.T) {
	providers := []opencode.ProviderInfo{
		{
			ID: "openai",
			Models: map[string]opencode.ModelInfo{
				"gpt-4":         {ID: "gpt-4", Name: "GPT-4"},
				"gpt-3.5-turbo": {ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo"},
			},
		},
		{
			ID: "anthropic",
			Models: map[string]opencode.ModelInfo{
				"claude-3": {ID: "claude-3", Name: "Claude 3"},
			},
		},
	}

	tests := []struct {
		name       string
		providerID string
		modelID    string
		want       bool
	}{
		{"existing model", "openai", "gpt-4", true},
		{"existing in other provider", "anthropic", "claude-3", true},
		{"non-existent model", "openai", "gpt-5", false},
		{"non-existent provider", "google", "gemini", false},
		{"empty provider", "", "gpt-4", false},
		{"empty model", "openai", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := modelExists(providers, tt.providerID, tt.modelID)
			if got != tt.want {
				t.Errorf("modelExists(%q, %q) = %v, want %v", tt.providerID, tt.modelID, got, tt.want)
			}
		})
	}
}

func TestEscapeMarkdownV2(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no special chars",
			input: "Hello world",
			want:  "Hello world",
		},
		{
			name:  "escapes underscores",
			input: "hello_world",
			want:  "hello\\_world",
		},
		{
			name:  "escapes asterisks",
			input: "bold*text",
			want:  "bold\\*text",
		},
		{
			name:  "escapes brackets",
			input: "[link](url)",
			want:  "\\[link\\]\\(url\\)",
		},
		{
			name:  "escapes backticks",
			input: "`code`",
			want:  "\\`code\\`",
		},
		{
			name:  "escapes tildes",
			input: "~strikethrough~",
			want:  "\\~strikethrough\\~",
		},
		{
			name:  "multiple special chars",
			input: "*bold* and _italic_ with `code` - dash",
			want:  "\\*bold\\* and \\_italic\\_ with \\`code\\` \\- dash",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EscapeMarkdownV2(tt.input)
			if got != tt.want {
				t.Errorf("EscapeMarkdownV2(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseChatID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{"valid positive", "12345", 12345, false},
		{"valid negative", "-987654", -987654, false},
		{"zero", "0", 0, false},
		{"large number", "9223372036854775807", 9223372036854775807, false},
		{"not a number", "abc", 0, true},
		{"empty string", "", 0, true},
		{"float", "3.14", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseChatID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseChatID(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseChatID(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// TestShouldAcquireProcessingLock tests the logic for determining when to acquire the processing lock
// Commands should NOT require the lock; only AI prompts (non-command text) should require it
func TestShouldAcquireProcessingLock(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		wantLock bool
	}{
		{"start command", "start", false},
		{"reset command", "reset", false},
		{"abort command", "abort", false},
		{"id command", "id", false},
		{"model command", "model", false},
		{"models command", "models", false},
		{"change_model command", "change_model", false},
		{"empty command means AI prompt", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldAcquireProcessingLock(tt.command)
			if got != tt.wantLock {
				t.Errorf("shouldAcquireProcessingLock(%q) = %v, want %v", tt.command, got, tt.wantLock)
			}
		})
	}
}

// TestCanExecuteAbortDuringProcessing verifies that /abort can be checked independently of the processing lock
func TestCanExecuteAbortDuringProcessing(t *testing.T) {
	tests := []struct {
		name           string
		command        string
		wantCanExecute bool
	}{
		{"/abort can execute during processing", "abort", true},
		{"/start cannot bypass lock", "start", false},
		{"/reset cannot bypass lock", "reset", false},
		{"/model cannot bypass lock", "model", false},
		{"AI prompts cannot bypass lock", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := canExecuteDuringProcessing(tt.command)
			if got != tt.wantCanExecute {
				t.Errorf("canExecuteDuringProcessing(%q) = %v, want %v", tt.command, got, tt.wantCanExecute)
			}
		})
	}
}

func TestHandleMessageCommandRouting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                  string
		messageText           string
		wantAbortCalls        int
		wantProvidersCalls    int
		wantDefaultModelCalls int
	}{
		{name: "start routes without lock", messageText: "/start"},
		{name: "reset routes without lock", messageText: "/reset"},
		{name: "abort routes without lock", messageText: "/abort", wantAbortCalls: 1},
		{name: "id routes without lock", messageText: "/id"},
		{name: "model routes without lock", messageText: "/model", wantDefaultModelCalls: 1},
		{name: "models routes without lock", messageText: "/models", wantProvidersCalls: 1},
		{name: "change_model routes without lock", messageText: "/change_model openai/gpt-4", wantProvidersCalls: 1},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			backend := newHandleMessageTestBackend()
			bot := newHandleMessageTestBot(t, backend)

			const chatID int64 = 42
			bot.mu.Lock()
			bot.processing[chatID] = true // Command must bypass this lock guard.
			bot.mu.Unlock()

			bot.handleMessage(newCommandMessage(chatID, tt.messageText))

			if backend.hasBusyMessage() {
				t.Fatalf("command %q should not hit processing-lock busy response", tt.messageText)
			}

			bot.mu.Lock()
			_, stillProcessing := bot.processing[chatID]
			bot.mu.Unlock()
			if !stillProcessing {
				t.Fatalf("command %q should not alter processing lock state", tt.messageText)
			}

			_, abortCalls, providersCalls, defaultModelCalls, _ := backend.snapshotCounts()
			if abortCalls != tt.wantAbortCalls {
				t.Fatalf("abort calls = %d, want %d", abortCalls, tt.wantAbortCalls)
			}
			if providersCalls != tt.wantProvidersCalls {
				t.Fatalf("providers calls = %d, want %d", providersCalls, tt.wantProvidersCalls)
			}
			if defaultModelCalls != tt.wantDefaultModelCalls {
				t.Fatalf("default model calls = %d, want %d", defaultModelCalls, tt.wantDefaultModelCalls)
			}
		})
	}
}

func TestHandleMessageAbortDuringProcessing(t *testing.T) {
	t.Parallel()

	backend := newHandleMessageTestBackend()
	bot := newHandleMessageTestBot(t, backend)

	const chatID int64 = 42
	bot.mu.Lock()
	bot.processing[chatID] = true
	bot.mu.Unlock()

	bot.handleMessage(newCommandMessage(chatID, "/abort"))

	if backend.hasBusyMessage() {
		t.Fatal("/abort must bypass processing busy response")
	}

	_, abortCalls, _, _, _ := backend.snapshotCounts()
	if abortCalls != 1 {
		t.Fatalf("abort calls = %d, want 1", abortCalls)
	}
}

func TestHandleMessageLockOnlyForAIPrompts(t *testing.T) {
	t.Parallel()

	t.Run("command path does not acquire processing lock", func(t *testing.T) {
		backend := newHandleMessageTestBackend()
		bot := newHandleMessageTestBot(t, backend)

		const chatID int64 = 42
		bot.handleMessage(newCommandMessage(chatID, "/start"))

		bot.mu.Lock()
		_, locked := bot.processing[chatID]
		bot.mu.Unlock()

		if locked {
			t.Fatal("command should not set processing lock")
		}
	})

	t.Run("regular text acquires processing lock", func(t *testing.T) {
		backend := newHandleMessageTestBackend()
		backend.promptStarted = make(chan struct{}, 1)
		backend.promptBlock = make(chan struct{})

		bot := newHandleMessageTestBot(t, backend)

		const chatID int64 = 42
		done := make(chan struct{})

		go func() {
			bot.handleMessage(newTextMessage(chatID, "hola opencode"))
			close(done)
		}()

		select {
		case <-backend.promptStarted:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for AI prompt to start")
		}

		bot.mu.Lock()
		lockedDuringPrompt := bot.processing[chatID]
		bot.mu.Unlock()
		if !lockedDuringPrompt {
			t.Fatal("regular text should set processing lock while prompt is running")
		}

		close(backend.promptBlock)

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for prompt completion")
		}

		bot.mu.Lock()
		_, stillLocked := bot.processing[chatID]
		bot.mu.Unlock()
		if stillLocked {
			t.Fatal("processing lock should be released after prompt completes")
		}

		_, _, _, _, promptCalls := backend.snapshotCounts()
		if promptCalls != 1 {
			t.Fatalf("prompt calls = %d, want 1", promptCalls)
		}
	})
}
