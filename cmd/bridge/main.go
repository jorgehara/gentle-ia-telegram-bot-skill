package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/jorgehara/go-telegram-opencode-bridge/internal/config"
	"github.com/jorgehara/go-telegram-opencode-bridge/internal/opencode"
	"github.com/jorgehara/go-telegram-opencode-bridge/internal/telegram"
)

// LoadEnvFile carga un archivo .env si existe
func LoadEnvFile() {
	// Intentar cargar .env del directorio actual
	if err := godotenv.Load(); err != nil {
		// No es error, simplemente no hay archivo .env
		log.Println("ℹ️  No se encontró archivo .env, usando variables de entorno del sistema")
	}
}

func main() {
	log.Println("🚀 Starting Go Telegram OpenCode Bridge...")

	// Cargar archivo .env si existe
	LoadEnvFile()

	// Cargar configuración
	cfg := config.LoadConfig()
	log.Printf("📋 Config loaded: Bot=%s..., OpenCode=%s", cfg.BotToken[:10], cfg.OpencodeURL)

	// Crear cliente OpenCode
	opencodeClient := opencode.NewOpencodeClient(cfg.OpencodeURL, cfg.OpencodeUsername, cfg.OpencodePassword)

	// Verificar conexión con OpenCode
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := opencodeClient.HealthCheck(ctx); err != nil {
		log.Printf("⚠️  Warning: OpenCode server not reachable: %v", err)
		log.Println("   El bridge continuará pero puede que no funcione correctamente")
	}

	// Crear servidor Telegram
	telegramBot, err := telegram.NewTelegramBot(cfg.BotToken, opencodeClient, cfg)
	if err != nil {
		log.Fatalf("❌ Error al inicializar Telegram bot: %v", err)
	}

	log.Printf("✅ Bot @%s inicializado correctamente", telegramBot.Bot.Self.UserName)

	// Iniciar polling de mensajes
	go telegramBot.StartPolling()

	// Mantener el proceso vivo
	log.Println("✅ Bridge corriendo. Presiona Ctrl+C para detener.")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Deteniendo bridge...")
	telegramBot.Stop()
	log.Println("✅ Bridge detenido correctamente")
}
