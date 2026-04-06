# gentle-ia-telegram-bot-skill

> 🤖 Un bridge de Telegram para OpenCode implementado en Go - listo para integrar como skill de gentle-ia

Este proyecto permite conectar un bot de Telegram con el servidor de OpenCode, creando un asistente de IA accesible desde Telegram. Está diseñado para ser portable, seguro y fácil de configurar.

---

## 📋 Requisitos del Sistema

### Software Necesario

| Requisito | Versión Mínima | Descripción |
|-----------|----------------|-------------|
| **Go** | 1.21+ | Compilador y entorno de desarrollo para Go |
| **OpenCode** | Latest | Servidor de IA local (debe estar corriendo) |
| **Telegram Bot Token** | - | Token del bot obtenido de @BotFather |
| **Sistema Operativo** | Windows 10+ / Linux / macOS | Compatible con los tres principales |

### Hardware Recomendado

- **RAM**: 4 GB mínimo, 8 GB recomendado
- **CPU**: Procesador moderno con al menos 2 núcleos
- **Espacio en disco**: 500 MB (incluyendo dependencias de Go)
- **Conexión a internet**: Necesaria para comunicarse con los servidores de Telegram y tus proveedores de IA

### Dependencias de red

El bridge necesita acceso a:
- `api.telegram.org` - Para comunicarse con el Bot API
- `localhost:4096` (o el puerto que uses) - Para conectar con OpenCode

---

## 🚀 Guía de Instalación

### Paso 1: Instalar Go

**Windows:**
1. Descarga Go desde [go.dev/dl](https://go.dev/dl)
2. Ejecuta el instalador y sigue los pasos
3. Verifica la instalación:
   ```cmd
   go version
   ```

**Linux (Debian/Ubuntu):**
```bash
sudo apt update
sudo apt install golang-go
```

**macOS:**
```bash
brew install go
```

### Paso 2: Obtener un Bot de Telegram

1. Abre **Telegram** y busca **@BotFather**
2. Envía el comando `/newbot`
3. Dale un nombre a tu bot (ej: `MiAsistenteBot`)
4. Dale un username que termine en `Bot` (ej: `mi_asistente_bot`)
5. **Copia el token** que te da BotFather (algo como `123456:ABC-DEF...`)

> ⚠️ **Importante**: Guarda este token en un lugar seguro. Es la clave de acceso a tu bot.

### Paso 3: Instalar OpenCode

```bash
# Instala globalmente con npm
npm install -g opencode-ai

# Verifica la instalación
opencode --version
```

### Paso 4: Clonar el Proyecto

```bash
git clone https://github.com/jorgehara/gentle-ia-telegram-bot-skill.git
cd gentle-ia-telegram-bot-skill
```

### Paso 5: Configurar Variables de Entorno

Crea un archivo `.env` en la raíz del proyecto:

```env
# === CONFIGURACIÓN REQUERIDA ===

# Token de tu bot de Telegram (obtenido de @BotFather)
TELEGRAM_BOT_TOKEN=tu_token_aqui

# === CONFIGURACIÓN OPCIONAL ===

# URL del servidor de OpenCode (default: http://localhost:4096)
OPENCODE_URL=http://localhost:4096

# Autenticación de OpenCode (si tienes密码 configurada)
# OPENCODE_USERNAME=opencode
# OPENCODE_PASSWORD=tu_password

# Directorio del proyecto para las sesiones de OpenCode
OPENCODE_PROJECT_DIR=.

# Puerto HTTP del bridge (default: 8080)
BRIDGE_PORT=8080

# Habilitar formato Markdown en respuestas (default: true)
ENABLE_MARKDOWN=true

# Modo debug (default: false)
DEBUG=false

# === SEGURIDAD (OPCIONAL) ===

# Lista de Chat IDs permitidos (vacío = permitir todos)
# Obtén tu ID enviando /id al bot una vez levantado
# ALLOWED_CHAT_IDS=123456789,987654321
```

### Paso 6: Compilar el Proyecto

```bash
# En Windows
go build -o gentle-ia-telegram-bot-skill.exe .

# En Linux/macOS
go build -o gentle-ia-telegram-bot-skill .
```

Esto genera un **binario único** que contiene todo el código compilado.

### Paso 7: Iniciar los Servicios

Necesitas ejecutar **dos componentes**:

1. **OpenCode Server** (en una terminal):
   ```bash
   opencode serve
   ```

2. **El Bridge** (en otra terminal):
   ```bash
   # Windows
   .\gentle-ia-telegram-bot-skill.exe

   # Linux/macOS
   ./gentle-ia-telegram-bot-skill
   ```

### Paso 8: Probar el Bot

1. Busca tu bot en Telegram por el username que configuraste
2. Envía `/start` para ver el mensaje de bienvenida
3. Envía cualquier texto para chatear con OpenCode

---

## ⚙️ Configuración Detallada

### Variables de Entorno

| Variable | Requerido | Default | Descripción |
|----------|-----------|---------|-------------|
| `TELEGRAM_BOT_TOKEN` | ✅ Sí | - | Token del bot de Telegram |
| `OPENCODE_URL` | No | `http://localhost:4096` | URL del servidor de OpenCode |
| `OPENCODE_USERNAME` | No | `opencode` | Usuario para autenticación básica |
| `OPENCODE_PASSWORD` | No | (vacío) | Contraseña para autenticación básica |
| `ALLOWED_CHAT_IDS` | No | (vacío) | Lista de IDs separados por coma |
| `OPENCODE_PROJECT_DIR` | No | `.` | Directorio de trabajo para OpenCode |
| `BRIDGE_PORT` | No | `8080` | Puerto HTTP del bridge |
| `ENABLE_MARKDOWN` | No | `true` | Habilitar formato Markdown |
| `DEBUG` | No | `false` | Habilitar logs de debug |

### Seguridad - Lista Blanca de Usuarios

Para restringir el acceso solo a usuarios específicos:

1. Inicia el bridge sin restricciones (deja `ALLOWED_CHAT_IDS` vacío)
2. Envía `/id` al bot para obtener tu Chat ID
3. Agrega el ID al archivo `.env`:
   ```
   ALLOWED_CHAT_IDS=123456789
   ```
4. Reinicia el bridge

### Autenticación de OpenCode

Si tienes密码 configurada en tu servidor de OpenCode:

```env
OPENCODE_USERNAME=tu_usuario
OPENCODE_PASSWORD=tu_password
```

---

## 🏗️ Arquitectura del Sistema

```
┌─────────────────────────────────────────────────────────────────────┐
│                         USUARIO EN TELEGRAM                         │
│                    (@mi_bot / mensaje de texto)                    │
└────────────────────────────────┬────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────────┐
│                     TELEGRAM BOT API                                │
│                   (Long Polling o Webhook)                         │
└────────────────────────────────┬────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────────┐
│                  gentle-ia-telegram-bot-skill                        │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────────────────┐   │
│  │   Config     │  │  Telegram    │  │     OpenCode Client    │   │
│  │  (.env)      │  │   Handler    │  │  (HTTP REST + Sessions)│   │
│  └──────────────┘  └──────────────┘  └────────────────────────┘   │
│         │                  │                      │                 │
│         │           ┌──────┴──────┐              │                 │
│         │           │  Session    │              │                 │
│         │           │   Manager   │              │                 │
│         │           └─────────────┘              │                 │
└─────────┼────────────────────────────────────────┼──────────────────┘
          │                                        │
          ▼                                        ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        OPENCODE SERVER                              │
│                   (Puerto 4096 por defecto)                         │
│  ┌─────────────┐  ┌─────────────┐  ┌────────────────────────────┐  │
│  │   Sessions  │  │   LLM Agent │  │      Tools (Edit/Read)     │  │
│  └─────────────┘  └─────────────┘  └────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

### Flujo de un Mensaje

1. **Usuario envía mensaje** → Telegram Bot API recibe el update
2. **Bridge procesa** → Valida si el usuario está en la lista blanca
3. **Crear/Obtener sesión** → Bridge gestiona una sesión por chat ID
4. **Enviar a OpenCode** → POST a `/session/{id}/message` con el prompt
5. **Recibir respuesta** → OpenCode procesa y devuelve el texto
6. **Enviar a Telegram** → Bridge usa `sendMessage` para responder

### Componentes Principales

| Archivo | Responsabilidad |
|---------|-----------------|
| `main.go` | Punto de entrada, inicialización, graceful shutdown |
| `config.go` | Carga y validación de variables de entorno |
| `telegram.go` | Handlers de Telegram Bot API, comandos, mensajes |
| `opencode.go` | Cliente HTTP para la API de OpenCode |

---

## 📖 Comandos Disponibles

| Comando | Descripción |
|---------|-------------|
| `/start` | Mensaje de bienvenida e información del bot |
| `/id` | Muestra el Chat ID del usuario (para configurar whitelist) |
| `/reset` | Reinicia la sesión de OpenCode |
| `/abort` | Cancela la operación actual en progreso |
| *(cualquier texto)* | Enviado a OpenCode para procesar con IA |

---

## 🔧 Scripts de Automatización

### start-stack.bat (Windows)

Script que inicia automáticamente OpenCode + el Bridge en dos terminales separadas.

### add-startup.ps1 (Windows)

Agrega el script de inicio al startup de Windows para que se ejecute automáticamente al encender la PC.

---

## 📦 Distribución y Deployment

### Binarios Pre-compilados

Para distribuir el proyecto sin que el usuario necesite Go:

1. **Compila el binario** para cada plataforma:
   ```bash
   # Windows
   GOOS=windows GOARCH=amd64 go build -o gentle-ia-telegram-bot-skill.exe .

   # Linux
   GOOS=linux GOARCH=amd64 go build -o gentle-ia-telegram-bot-skill .

   # macOS
   GOOS=darwin GOARCH=amd64 go build -o gentle-ia-telegram-bot-skill-macos .
   ```

2. **Crea un ZIP** con:
   - El binario compilado
   - El archivo `.env.example`
   - Un README simplificado

### Integración con gentle-ia

Este proyecto está diseñado para integrarse como un **skill** de gentle-ia:

1. El skill puede generar automáticamente la estructura de archivos
2. Configurar las variables de entorno
3. Compilar y deploying el binario

---

## 🐛 Solución de Problemas

### "OpenCode server not reachable"

- Verifica que `opencode serve` esté ejecutándose
- Confirma que el puerto en `OPENCODE_URL` sea correcto (default: 4096)
- Verifica el firewall de Windows

### "Bad Request: can't parse entities"

- El texto de respuesta contiene caracteres especiales de Markdown
- Esto ya está manejado por el bridge (escape automático)

### "Chat ID no autorizado"

- Verifica que tu Chat ID esté en `ALLOWED_CHAT_IDS`
- Envía `/id` al bot para confirmar tu ID

### El bot no responde

- Verifica el token de Telegram
- Confirma que el bot esté activo (buscalo en Telegram)
- Revisa los logs en la terminal

---

## 📝 Notas de Implementación

### Decisiones de Diseño

1. **Long Polling** en lugar de Webhook:
   - Más simple de configurar
   - No requiere HTTPS
   - Funciona en localhost
   - Webhook no está implementado actualmente

2. **Una sesión por chat**:
   - Mantiene contexto de conversación
   - Persiste en memoria (se limpia al reiniciar)

3. **MarkdownV2**:
   - Mejor formato que HTML
   - Requiere escaping de caracteres especiales

4. **Context en todas las HTTP calls**:
   - Previene race conditions
   - Permite timeouts adecuados

### Limitaciones Conocidas

- Las sesiones se pierden al reiniciar el bridge
- No soporta adjuntos de archivos (solo texto)
- Un solo mensaje a la vez por chat (no paralelo)

---

## 🔄 Actualizaciones y Mantenimiento

Para actualizar a una nueva versión:

```bash
# Actualizar código
git pull origin master

# Recompilar
go build -o gentle-ia-telegram-bot-skill.exe .

# Reiniciar los servicios
```

---

## 📄 Licencia

MIT License - Ver archivo [LICENSE](LICENSE)

---

## 🤝 Contribuciones

¡Las contribuciones son bienvenidas! Para contribuir:

1. Haz fork del repositorio
2. Crea una rama (`feature/tu-mejora`)
3. Commitea tus cambios
4. Abre un Pull Request

---

## 🛠️ Tecnologías Usadas

| Tecnología | Propósito |
|------------|-----------|
| **Go 1.21+** | Lenguaje de programación principal |
| **go-telegram-bot-api** | Wrapper de Telegram Bot API para Go |
| **godotenv** | Carga de variables de entorno desde .env |
| **OpenCode** | Servidor de IA backend |
| **Telegram Bot API** | Plataforma de mensajería |

---

## 📚 Recursos Externos

- [Documentación de Telegram Bot API](https://core.telegram.org/bots/api)
- [OpenCode Official Docs](https://opencode.ai/docs/)
- [Go Documentation](https://go.dev/doc/)
- [Wiki de @BotFather](https://t.me/BotFather)

---

*Este proyecto es parte del ecosistema gentle-ia - Asistentes de IA personalizados y auto-hosteados.*