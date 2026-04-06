package telegram

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	tg "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/jorgehara/go-telegram-opencode-bridge/internal/config"
	"github.com/jorgehara/go-telegram-opencode-bridge/internal/opencode"
)

// TelegramBot maneja la comunicación con Telegram
type TelegramBot struct {
	Bot    *tg.BotAPI
	Client *opencode.OpencodeClient
	Config *config.Config
	// chatModels stores model selections per chat ID (in-memory)
	chatModels map[int64]*opencode.ModelRef
	mu         sync.Mutex
	// Processing indica si hay un mensaje siendo procesado por chat
	processing map[int64]bool
	stopChan   chan struct{}
}

// NewTelegramBot crea e inicializa el bot de Telegram
func NewTelegramBot(token string, client *opencode.OpencodeClient, cfg *config.Config) (*TelegramBot, error) {
	bot, err := tg.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	bot.Debug = cfg.Debug

	return &TelegramBot{
		Bot:        bot,
		Client:     client,
		Config:     cfg,
		chatModels: make(map[int64]*opencode.ModelRef),
		processing: make(map[int64]bool),
		stopChan:   make(chan struct{}),
	}, nil
}

// StartPolling inicia el polling de mensajes
func (b *TelegramBot) StartPolling() {
	log.Println("📡 Iniciando polling de mensajes...")

	updateConfig := tg.NewUpdate(0)
	updateConfig.Timeout = 60

	// Usar webhook opcional (descomenta si preferís webhook)
	// b.StartWebhook()
	// return

	updates := b.Bot.GetUpdatesChan(updateConfig)

	for {
		select {
		case <-b.stopChan:
			log.Println("⏹️ Polling detenido")
			return
		case update := <-updates:
			if update.Message != nil {
				go b.handleMessage(update.Message)
			}
			if update.CallbackQuery != nil {
				go b.handleCallback(update.CallbackQuery)
			}
		}
	}
}

// Stop detiene el polling
func (b *TelegramBot) Stop() {
	close(b.stopChan)
	b.Bot.StopReceivingUpdates()
}

// handleMessage procesa mensajes entrantes
func (b *TelegramBot) handleMessage(msg *tg.Message) {
	// Ignorar mensajes sin texto
	if msg.Text == "" && msg.Command() == "" {
		return
	}

	chatID := msg.Chat.ID
	text := msg.Text

	// Verificar si el chat está en la whitelist
	if !b.Config.IsAllowedChat(chatID) {
		log.Printf("🚫 Chat ID %d no autorizado (usuario: @%s)", chatID, msg.From.UserName)
		b.sendMessage(chatID, "🚫 Lo siento, este bot es de uso privado.")
		return
	}

	// PHASE 2 REFACTOR: Move command routing BEFORE lock acquisition
	command := msg.Command()

	// Handle commands that don't need the processing lock
	if command != "" {
		// /abort is special - it can execute even when lock is held
		if command == "abort" {
			// Get or create session for abort command
			sessionCtx, sessionCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer sessionCancel()
			sessionID, err := b.Client.GetOrCreateSession(sessionCtx, chatID, b.Config.ProjectDir)
			if err != nil {
				log.Printf("❌ Error creando sesión para abort en chat %d: %v", chatID, err)
				b.sendMessage(chatID, "❌ Error al conectar con OpenCode para abortar.")
				return
			}
			b.handleAbort(sessionID, chatID)
			return
		}

		// Other commands execute without lock
		if command == "start" {
			b.sendWelcomeMessage(chatID)
			return
		}
		if command == "reset" {
			b.handleReset(chatID)
			return
		}
		if command == "id" {
			b.handleGetID(msg)
			return
		}
		if command == "model" {
			b.handleModel(chatID)
			return
		}
		if command == "models" {
			b.handleModels(chatID, text)
			return
		}
		if command == "change_model" {
			b.handleChangeModel(chatID, text)
			return
		}
	}

	// Non-command messages (AI prompts) require the processing lock
	// Verificar si ya está procesando
	b.mu.Lock()
	if b.processing[chatID] {
		b.mu.Unlock()
		// Responder que está ocupado
		b.sendMessage(chatID, "⏳ Estoy procesando tu mensaje anterior. Esperá un momento...")
		return
	}
	b.processing[chatID] = true
	b.mu.Unlock()

	// Liberar lock al terminar
	defer func() {
		b.mu.Lock()
		delete(b.processing, chatID)
		b.mu.Unlock()
	}()

	// Obtener o crear sesión (timeout corto para operación rápida)
	sessionCtx, sessionCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer sessionCancel()
	sessionID, err := b.Client.GetOrCreateSession(sessionCtx, chatID, b.Config.ProjectDir)
	if err != nil {
		log.Printf("❌ Error creando sesión para chat %d: %v", chatID, err)
		b.sendMessage(chatID, "❌ Error al conectar con OpenCode. Probá más tarde.")
		return
	}

	// Si no es comando local, enviar a OpenCode
	prompt := text

	// Indicar que está procesando
	b.sendChatAction(chatID, tg.ChatTyping)

	// Enviar a OpenCode con timeout largo para prompts complejos
	promptCtx, promptCancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer promptCancel()

	// Mensajes de progreso cada 2 minutos
	progressDone := make(chan bool)
	go b.sendProgressMessages(chatID, progressDone)

	// Obtener modelo seleccionado para este chat
	b.mu.Lock()
	modelRef := b.chatModels[chatID]
	b.mu.Unlock()

	response, err := b.Client.SendPromptWithModel(promptCtx, sessionID, prompt, modelRef)

	// Detener mensajes de progreso
	close(progressDone)

	if err != nil {
		log.Printf("❌ Error en prompt para chat %d: %v", chatID, err)
		b.sendMessage(chatID, "❌ Error al procesar tu mensaje: "+err.Error())
		return
	}

	// Responder al usuario
	if b.Config.EnableMarkdown {
		b.sendMessageWithMarkdown(chatID, response)
	} else {
		b.sendMessage(chatID, response)
	}
}

// sendMessage envía un mensaje simple
func (b *TelegramBot) sendMessage(chatID int64, text string) {
	msg := tg.NewMessage(chatID, text)
	msg.DisableWebPagePreview = true
	if _, err := b.Bot.Send(msg); err != nil {
		log.Printf("❌ Error enviando mensaje: %v", err)
	}
}

// sendMessageWithMarkdown envía mensaje con soporte markdown
func (b *TelegramBot) sendMessageWithMarkdown(chatID int64, text string) {
	// Escapar caracteres especiales de MarkdownV2
	escapedText := EscapeMarkdownV2(text)

	msg := tg.NewMessage(chatID, escapedText)
	msg.ParseMode = "MarkdownV2"
	msg.DisableWebPagePreview = true
	if _, err := b.Bot.Send(msg); err != nil {
		// Si falla con markdown, intentar sin formato
		log.Printf("⚠️ Markdown falló, enviando sin formato: %v", err)
		b.sendMessage(chatID, text)
	}
}

// sendChatAction envía acción de typing
func (b *TelegramBot) sendChatAction(chatID int64, action string) {
	chatAction := tg.NewChatAction(chatID, action)
	b.Bot.Request(chatAction)
}

// sendWelcomeMessage mensaje de inicio
func (b *TelegramBot) sendWelcomeMessage(chatID int64) {
	text := `🤖 *Bienvenido al Bridge de OpenCode*

Puedo ayudarte a usar OpenCode directamente desde Telegram\.

*Comandos disponibles:*
• /start \- Mostrar este mensaje
• /reset \- Reiniciar la sesión
• /abort \- Cancelar operación actual
• /model \- Ver modelo actual del chat
• /models \- Listar modelos disponibles
• /change\_model provider/model \- Cambiar modelo del chat
• /change\_model default \- Restaurar modelo por defecto
• Envía cualquier texto para conversar con OpenCode

_El proyecto se define en OPENCODE_PROJECT_DIR \(default: \.\)_

Powered by Go \+ OpenCode API`
	b.sendMessageWithMarkdown(chatID, text)
}

// handleReset reinicia la sesión
func (b *TelegramBot) handleReset(chatID int64) {
	b.Client.ClearSession(chatID)
	b.sendMessage(chatID, "🔄 Sesión reiniciada. Nueva sesión creada.")
}

// handleAbort cancela la operación actual
func (b *TelegramBot) handleAbort(sessionID string, chatID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := b.Client.AbortSession(ctx, sessionID); err != nil {
		b.sendMessage(chatID, "❌ Error al abortar: "+err.Error())
		return
	}
	b.sendMessage(chatID, "✅ Operación cancelada.")
}

// handleGetID muestra información del chat y usuario
func (b *TelegramBot) handleGetID(msg *tg.Message) {
	chatID := msg.Chat.ID
	userID := msg.From.ID
	username := msg.From.UserName
	firstName := msg.From.FirstName
	lastName := msg.From.LastName

	info := fmt.Sprintf("🆔 *Información del Chat*\n\n📱 Chat ID: `%d`\n👤 User ID: `%d`\n🏷️ Username: @%s\n📝 Nombre: %s %s\n\n_Usá el Chat ID para configurar ALLOWED\\_CHAT\\_IDS_",
		chatID,
		userID,
		username,
		firstName,
		lastName,
	)

	log.Printf("ℹ️ ID solicitado - Chat: %d, User: %d (@%s)", chatID, userID, username)
	b.sendMessageWithMarkdown(chatID, info)
}

// handleCallback maneja callbacks de botones inline
func (b *TelegramBot) handleCallback(callback *tg.CallbackQuery) {
	// Por ahora simplemente responder
	answer := tg.NewCallback(callback.ID, "Recibido")
	b.Bot.Request(answer)

	// Aquí se pueden manejar botones de permisos, etc.
}

// SendTypingIndicator envía indicador de "escribiendo"
func (b *TelegramBot) SendTypingIndicator(chatID int64) {
	for {
		select {
		case <-b.stopChan:
			return
		default:
			b.sendChatAction(chatID, tg.ChatTyping)
			time.Sleep(3 * time.Second)
		}
	}
}

// EscapeMarkdownV2 escapa caracteres especiales para MarkdownV2
// Escapa los caracteres requeridos por la API de Telegram, incluyendo el '-'
func EscapeMarkdownV2(text string) string {
	replacer := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
		"~", "\\~",
		"`", "\\`",
		">", "\\>",
		"#", "\\#",
		"+", "\\+",
		"-", "\\-",
		"=", "\\=",
		"|", "\\|",
		"{", "\\{",
		"}", "\\}",
		".", "\\.",
		"!", "\\!",
	)
	return replacer.Replace(text)
}

// parseChatID convierte string a int64
func ParseChatID(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// sendProgressMessages envía mensajes de progreso cada 2 minutos mientras procesa
func (b *TelegramBot) sendProgressMessages(chatID int64, done chan bool) {
	messages := []string{
		"⏳ Estoy trabajando en tu solicitud, dame un momento...",
		"⏳ Todavía procesando, casi listo...",
		"⏳ Gracias por tu paciencia, sigo trabajando en esto...",
		"⏳ Un poco más de tiempo, la respuesta está en camino...",
	}

	ticker := time.NewTicker(120 * time.Second) // Cada 2 minutos
	defer ticker.Stop()

	messageIndex := 0

	for {
		select {
		case <-done:
			// Procesamiento terminado, salir
			return
		case <-ticker.C:
			// Enviar mensaje de progreso
			if messageIndex < len(messages) {
				b.sendMessage(chatID, messages[messageIndex])
				messageIndex++
			} else {
				// Si ya enviamos todos, repetir el último
				b.sendMessage(chatID, messages[len(messages)-1])
			}
		}
	}
}

// handleModel muestra el modelo actual del chat.
func (b *TelegramBot) handleModel(chatID int64) {
	b.mu.Lock()
	modelRef := b.chatModels[chatID]
	b.mu.Unlock()

	if modelRef != nil {
		b.sendMessage(chatID, fmt.Sprintf("📋 Modelo: %s/%s", modelRef.ProviderID, modelRef.ModelID))
		return
	}

	// Sin override: consultar modelo default del servidor
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	defaultModel, err := b.Client.GetDefaultModel(ctx)
	if err != nil {
		b.sendMessage(chatID, "📋 Modelo: default (sin override) — no se pudo consultar el servidor: "+err.Error())
		return
	}

	if defaultModel != nil && defaultModel.ModelID != "" {
		b.sendMessage(chatID, fmt.Sprintf("📋 Modelo: default del servidor → %s/%s (sin override en este chat)", defaultModel.ProviderID, defaultModel.ModelID))
		return
	}

	b.sendMessage(chatID, "📋 Modelo: default (sin override y sin modelo configurado en el servidor)")
}

// handleModels lista los modelos disponibles con paginación por comando.
func (b *TelegramBot) handleModels(chatID int64, rawText string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	providers, err := b.Client.GetProviders(ctx)
	if err != nil {
		b.sendMessage(chatID, "❌ Error al obtener modelos: "+err.Error())
		return
	}

	// Construir lista plana de modelos
	type modelEntry struct {
		identifier string
		name       string
	}
	var allModels []modelEntry
	for _, p := range providers {
		if len(p.Models) == 0 {
			continue
		}
		for _, m := range p.Models {
			allModels = append(allModels, modelEntry{
				identifier: fmt.Sprintf("%s/%s", p.ID, m.ID),
				name:       m.Name,
			})
		}
	}

	// Sort models alphabetically by identifier to ensure consistent pagination
	sort.Slice(allModels, func(i, j int) bool {
		return allModels[i].identifier < allModels[j].identifier
	})

	if len(allModels) == 0 {
		b.sendMessage(chatID, "📭 No hay modelos disponibles.")
		return
	}

	// Parsear página del comando: /models, /models 2, etc.
	page := 1
	parts := strings.Fields(rawText)
	if len(parts) >= 2 {
		if p, err := strconv.Atoi(parts[1]); err == nil && p > 0 {
			page = p
		}
	}

	const perPage = 10
	totalPages := (len(allModels) + perPage - 1) / perPage
	if page > totalPages {
		page = totalPages
	}

	start := (page - 1) * perPage
	end := start + perPage
	if end > len(allModels) {
		end = len(allModels)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📚 Modelos disponibles (página %d/%d):\n\n", page, totalPages))

	for i := start; i < end; i++ {
		sb.WriteString(fmt.Sprintf("%d. `%s` — %s\n", i+1, allModels[i].identifier, allModels[i].name))
	}

	sb.WriteString(fmt.Sprintf("\n💡 Usá /change_model provider/model para seleccionar"))
	if totalPages > 1 {
		if page < totalPages {
			sb.WriteString(fmt.Sprintf("\n📄 Siguiente: /models %d", page+1))
		}
		if page > 1 {
			sb.WriteString(fmt.Sprintf("\n📄 Anterior: /models %d", page-1))
		}
	}

	b.sendMessageWithMarkdown(chatID, sb.String())
}

// parseModelRef parses "provider/model" into (providerID, modelID) or returns an error message.
func parseModelRef(value string) (providerID, modelID, errMsg string) {
	slashIdx := strings.Index(value, "/")
	if slashIdx == -1 {
		return "", "", "⚠️ Formato inválido. Usá: /change_model provider/model\nEjemplo: /change_model openai/gpt-4"
	}
	providerID = value[:slashIdx]
	modelID = value[slashIdx+1:]
	if providerID == "" || modelID == "" {
		return "", "", "⚠️ Provider y model no pueden estar vacíos. Usá: /change_model provider/model"
	}
	return providerID, modelID, ""
}

// modelExists checks if providerID/modelID exists in the providers list.
func modelExists(providers []opencode.ProviderInfo, providerID, modelID string) bool {
	for _, p := range providers {
		if p.ID == providerID {
			_, exists := p.Models[modelID]
			return exists
		}
	}
	return false
}

// handleChangeModel cambia el modelo del chat.
func (b *TelegramBot) handleChangeModel(chatID int64, rawText string) {
	// Parsear: /change_model provider/model  o  /change_model default
	parts := strings.Fields(rawText)
	if len(parts) < 2 {
		b.sendMessage(chatID, "⚠️ Uso: /change_model provider/model\nO bien: /change_model default")
		return
	}

	value := parts[1]

	// /change_model default → limpiar override
	if value == "default" {
		b.mu.Lock()
		delete(b.chatModels, chatID)
		b.mu.Unlock()
		b.sendMessage(chatID, "✅ Modelo restaurado al default del servidor.")
		return
	}

	// Parsear provider/model
	providerID, modelID, errMsg := parseModelRef(value)
	if errMsg != "" {
		b.sendMessage(chatID, errMsg)
		return
	}

	// Validar contra los proveedores/modelos reales del servidor
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	providers, err := b.Client.GetProviders(ctx)
	if err != nil {
		b.sendMessage(chatID, "❌ Error al validar modelo: "+err.Error())
		return
	}

	if !modelExists(providers, providerID, modelID) {
		b.sendMessage(chatID, fmt.Sprintf("❌ Modelo inválido: %s/%s\nUsá /models para ver los disponibles.", providerID, modelID))
		return
	}

	b.mu.Lock()
	b.chatModels[chatID] = &opencode.ModelRef{
		ProviderID: providerID,
		ModelID:    modelID,
	}
	b.mu.Unlock()

	b.sendMessage(chatID, fmt.Sprintf("✅ Modelo cambiado a: %s/%s", providerID, modelID))
}

// shouldAcquireProcessingLock determines if a message requires the processing lock.
// Commands (non-empty string) return false because they execute quickly and independently.
// AI prompts (empty command string) return true because they require exclusive processing.
func shouldAcquireProcessingLock(command string) bool {
	return command == ""
}

// canExecuteDuringProcessing checks if a command can execute even when another operation holds the processing lock.
// Only /abort can bypass the lock to allow canceling an in-progress AI prompt.
func canExecuteDuringProcessing(command string) bool {
	return command == "abort"
}
