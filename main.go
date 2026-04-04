package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
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
	cfg := LoadConfig()
	log.Printf("📋 Config loaded: Bot=%s..., OpenCode=%s", cfg.BotToken[:10], cfg.OpencodeURL)

	// Crear cliente OpenCode
	opencodeClient := NewOpencodeClient(cfg.OpencodeURL, cfg.OpencodeUsername, cfg.OpencodePassword)

	// Verificar conexión con OpenCode
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := opencodeClient.HealthCheck(ctx); err != nil {
		log.Printf("⚠️  Warning: OpenCode server not reachable: %v", err)
		log.Println("   El bridge continuará pero puede que no funcione correctamente")
	}

	// Crear servidor Telegram
	telegramBot, err := NewTelegramBot(cfg.BotToken, opencodeClient, cfg)
	if err != nil {
		log.Fatalf("❌ Error al inicializar Telegram bot: %v", err)
	}

	log.Printf("✅ Bot @%s inicializado correctamente", telegramBot.Bot.Self.UserName)

	// Iniciar servidor HTTP para callbacks (webhook mode opcional)
	go func() {
		port := cfg.HTTPPort
		log.Printf("🌐 Servidor HTTP escuchando en :%d", port)
		if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
			log.Printf("⚠️  Servidor HTTP terminó: %v", err)
		}
	}()

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
