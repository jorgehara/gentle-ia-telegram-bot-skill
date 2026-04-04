package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	// Telegram
	BotToken       string
	AllowedChatIDs []int64 // Lista de chat IDs permitidos (vacío = todos)

	// OpenCode
	OpencodeURL      string
	OpencodeUsername string
	OpencodePassword string

	// HTTP Server
	HTTPPort int

	// OpenCode Session
	ProjectDir string
	SessionTTL time.Duration

	// Features
	EnableMarkdown bool
	Debug          bool
}

func LoadConfig() *Config {
	// Cargar valores con defaults sensatos
	cfg := &Config{
		OpencodeURL:      getEnv("OPENCODE_URL", "http://localhost:4096"),
		OpencodeUsername: getEnv("OPENCODE_USERNAME", "opencode"),
		OpencodePassword: getEnv("OPENCODE_PASSWORD", ""),
		HTTPPort:         getEnvInt("BRIDGE_PORT", 8080),
		ProjectDir:       getEnv("OPENCODE_PROJECT_DIR", "."),
		SessionTTL:       getEnvDuration("SESSION_TTL", 24*time.Hour),
		EnableMarkdown:   getEnvBool("ENABLE_MARKDOWN", true),
		Debug:            getEnvBool("DEBUG", false),
		AllowedChatIDs:   getEnvInt64Slice("ALLOWED_CHAT_IDS", []int64{}),
	}

	// Bot token es obligatorio
	cfg.BotToken = os.Getenv("TELEGRAM_BOT_TOKEN")
	if cfg.BotToken == "" {
		log.Fatal("❌ TELEGRAM_BOT_TOKEN es requerido. Ejemplo: export TELEGRAM_BOT_TOKEN=123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11")
	}

	if cfg.Debug {
		log.SetFlags(log.Lshortfile | log.LstdFlags)
	}

	return cfg
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
		log.Printf("⚠️  Invalid value for %s, using default: %d", key, defaultValue)
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1" || value == "yes"
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if dur, err := time.ParseDuration(value); err == nil {
			return dur
		}
		log.Printf("⚠️  Invalid duration for %s, using default: %v", key, defaultValue)
	}
	return defaultValue
}

func getEnvInt64Slice(key string, defaultValue []int64) []int64 {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	parts := strings.Split(value, ",")
	result := make([]int64, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if id, err := strconv.ParseInt(part, 10, 64); err == nil {
			result = append(result, id)
		} else {
			log.Printf("⚠️  Invalid chat ID '%s' in %s, skipping", part, key)
		}
	}

	return result
}

func (c *Config) String() string {
	return fmt.Sprintf(`Config{
  BotToken: %s...
  OpencodeURL: %s
  OpencodeUsername: %s
  HTTPPort: %d
  ProjectDir: %s
  EnableMarkdown: %v
  AllowedChatIDs: %v
}`,
		c.BotToken[:min(len(c.BotToken), 10)]+"...",
		c.OpencodeURL,
		c.OpencodeUsername,
		c.HTTPPort,
		c.ProjectDir,
		c.EnableMarkdown,
		c.AllowedChatIDs,
	)
}

// IsAllowedChat verifica si un chat ID está en la whitelist
func (c *Config) IsAllowedChat(chatID int64) bool {
	// Si no hay restricciones, permitir todos
	if len(c.AllowedChatIDs) == 0 {
		return true
	}

	// Verificar si el chatID está en la lista
	for _, allowedID := range c.AllowedChatIDs {
		if allowedID == chatID {
			return true
		}
	}

	return false
}
