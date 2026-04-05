package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	tg "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// TelegramBot maneja la comunicación con Telegram
type TelegramBot struct {
	Bot    *tg.BotAPI
	Client *OpencodeClient
	Config *Config
	mu     sync.Mutex
	// Processing indica si hay un mensaje siendo procesado por chat
	processing map[int64]bool
	stopChan   chan struct{}
}

// NewTelegramBot crea e inicializa el bot de Telegram
func NewTelegramBot(token string, client *OpencodeClient, cfg *Config) (*TelegramBot, error) {
	bot, err := tg.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	bot.Debug = cfg.Debug

	return &TelegramBot{
		Bot:        bot,
		Client:     client,
		Config:     cfg,
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

	// Obtener o crear sesión
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	sessionID, err := b.Client.GetOrCreateSession(ctx, chatID, b.Config.ProjectDir)
	if err != nil {
		log.Printf("❌ Error creando sesión para chat %d: %v", chatID, err)
		b.sendMessage(chatID, "❌ Error al conectar con OpenCode. Probá más tarde.")
		return
	}

	// Determinar el texto a enviar
	// Solo procesar comandos locales si es un comando puro (sin argumentos extra)
	commandPrefix := "/" + msg.Command() + " "
	isLocalCommand := msg.Command() != "" && (text == commandPrefix || text == "/"+msg.Command())

	if isLocalCommand {
		command := msg.Command()
		if command == "start" {
			b.sendWelcomeMessage(chatID)
			return
		}
		if command == "reset" {
			b.handleReset(chatID)
			return
		}
		if command == "abort" {
			b.handleAbort(sessionID, chatID)
			return
		}
		if command == "id" {
			b.handleGetID(msg)
			return
		}
	}

	// Si no es comando local, enviar a OpenCode
	prompt := text

	// Indicar que está procesando
	b.sendChatAction(chatID, tg.ChatTyping)

	// Enviar a OpenCode
	response, err := b.Client.SendPrompt(ctx, sessionID, prompt)
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

Puedo ayudarte a usar OpenCode directamente desde Telegram.

*Comandos disponibles:*
• /start - Mostrar este mensaje
• /reset - Reiniciar la sesión
• /abort - Cancelar operación actual
• Envía cualquier texto para conversar con OpenCode

_El proyecto se define en OPENCODE_PROJECT_DIR (default: .)_

Powered by Go + OpenCode API`
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
