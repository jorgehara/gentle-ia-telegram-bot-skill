# gentle-ia telegram-bot-skill

> 🚀 A lightning-fast Telegram bot bridge for OpenCode, written in Go

Connect your Telegram bot to OpenCode's AI backend in seconds. Built with Go for speed and simplicity.

## ✨ Features

- 🔥 **Fast & Lightweight** - Single binary, minimal dependencies
- 🤖 **OpenCode Integration** - Direct connection to OpenCode server API
- 💬 **Full Telegram Support** - Commands, inline keyboards, markdown
- 🔒 **Session Management** - Persistent conversations per chat
- ⚙️ **Easy Configuration** - Simple `.env` file setup
- 🐳 **Production Ready** - Built-in error handling and graceful shutdown

## 📦 Installation

### Prerequisites

- Go 1.21+ (for building from source)
- [OpenCode](https://opencode.ai) installed and running
- Telegram Bot Token (get one from [@BotFather](https://t.me/BotFather))

### Option 1: Download Binary

Download the latest release from [Releases](https://github.com/jorgehara/gentle-ia-telegram-bot-skill/releases)

### Option 2: Build from Source

```bash
git clone https://github.com/jorgehara/gentle-ia-telegram-bot-skill.git
cd gentle-ia-telegram-bot-skill
go build -o gentle-ia-telegram-bot-skill .
```

## 🚀 Quick Start

1. **Create a Telegram bot** with [@BotFather](https://t.me/BotFather)

2. **Start OpenCode server**:
   ```bash
   opencode serve
   ```

3. **Configure the bridge**:
   ```bash
   cp .env.example .env
   # Edit .env and add your TELEGRAM_BOT_TOKEN
   ```

4. **Run the bridge**:
   ```bash
   ./gentle-ia-telegram-bot-skill
   ```

5. **Chat with your bot** on Telegram! 🎉

## ⚙️ Configuration

All configuration is done via environment variables or `.env` file:

```env
# Required
TELEGRAM_BOT_TOKEN=your_bot_token_here

# Security - Whitelist (empty = allow all)
# Get your Chat ID with /id command
ALLOWED_CHAT_IDS=123456789,987654321

# OpenCode Server (defaults)
OPENCODE_URL=http://localhost:4096
OPENCODE_USERNAME=opencode
OPENCODE_PASSWORD=

# Optional
OPENCODE_PROJECT_DIR=.
BRIDGE_PORT=8080
ENABLE_MARKDOWN=true
DEBUG=false
```

## 📖 Usage

### Commands

- `/start` - Welcome message and bot info
- `/id` - Get your Chat ID (for whitelist configuration)
- `/reset` - Reset the current session
- `/abort` - Cancel the current operation

Just send any message to chat with OpenCode!

### Security

Use `ALLOWED_CHAT_IDS` to restrict access to specific Telegram users:

1. Start the bot without restrictions
2. Send `/id` to get your Chat ID
3. Add your Chat ID to `.env`:
   ```env
   ALLOWED_CHAT_IDS=your_chat_id_here
   ```
4. Restart the bot

## 🏗️ Architecture

```
┌─────────────┐     HTTP      ┌─────────────┐     REST     ┌─────────────┐
│  Telegram   │ ◄──────────►  │  gentle-ia-telegram-bot-skill │ ◄─────────►  │  OpenCode   │
│    API      │               │    (Go)     │              │   Server    │
└─────────────┘               └─────────────┘              └─────────────┘
```

- **Telegram Bot API**: Long polling for updates
- **OpenCode Server**: HTTP REST API on port 4096
- **Session Management**: SQLite-based persistence per chat
- **Concurrency**: Goroutines for handling multiple chats

## 🛠️ Development

```bash
# Install dependencies
go mod download

# Run in development
go run .

# Build
go build -o gentle-ia-telegram-bot-skill .

# Run tests
go test ./...
```

## 📝 Project Structure

```
gentle-ia-telegram-bot-skill/
├── main.go          # Entry point and server setup
├── config.go        # Configuration loading
├── telegram.go      # Telegram bot handlers
├── opencode.go      # OpenCode API client
├── .env.example     # Example configuration
└── README.md
```

## 🤝 Contributing

Contributions are welcome! Feel free to:

- 🐛 Report bugs
- 💡 Suggest features
- 🔧 Submit pull requests

## 📄 License

MIT License - see [LICENSE](LICENSE) for details

## 🙏 Acknowledgments

- [OpenCode](https://opencode.ai) - The AI coding assistant
- [telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api) - Go Telegram Bot API wrapper

---

**Made with ❤️ by [Jorge Hara](https://github.com/jorgehara)**

*Part of the [Gentle AI](https://gentle-ia.com) ecosystem*