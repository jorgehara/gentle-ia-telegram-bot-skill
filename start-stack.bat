@echo off
REM ========================================
REM  gentle-ia telegram-bot-skill - Auto Start Script
REM  Inicia OpenCode + Bot automáticamente
REM ========================================

echo ========================================
echo  🚀 Iniciando gentle-ia telegram-bot-skill Stack
echo ========================================

REM Cambiar al directorio del proyecto
cd /d "%~dp0"

REM Verificar si OpenCode está en PATH
where opencode >nul 2>&1
if %ERRORLEVEL% NEQ 0 (
    echo ❌ OpenCode no encontrado en PATH
    echo    Asegurate de tener OpenCode instalado
    echo    PD: podes_installarlo con: npm install -g opencode-ai
    echo.
    pause
    exit /b 1
)

REM Verificar si existe el binario
if not exist "gentle-ia-telegram-bot-skill.exe" (
    echo ⚠️  Construyendo gentle-ia-telegram-bot-skill.exe...
    go build -o gentle-ia-telegram-bot-skill.exe .
    if %ERRORLEVEL% NEQ 0 (
        echo ❌ Error al compilar
        pause
        exit /b 1
    )
)

echo ✅ Verificaciones completadas
echo.

REM Abrir ventana de OpenCode Server
echo 📡 Iniciando OpenCode Server...
start "OpenCode Server" cmd /k "echo 🚀 OpenCode Server && opencode serve"

REM Esperar un momento para que OpenCode inicie
timeout /t 3 /nobreak >nul

REM Abrir ventana del Bot
echo 🤖 Iniciando gentle-ia telegram-bot-skill Bot...
start "gentle-ia Bot" cmd /k "echo 🤖 gentle-ia Bot && gentle-ia-telegram-bot-skill.exe"

echo.
echo ========================================
echo ✅ Stack iniciado correctamente!
echo.
echo  Terminales abiertas:
echo   - OpenCode Server (puerto 4096)
echo   - gentle-ia telegram-bot-skill Bot (puerto 8080)
echo.
echo  Chatea con tu bot en Telegram: @jorgeharadevs_gobot
echo ========================================

REM Esta ventana se cierra automaticamente
timeout /t 5 /nobreak >nul
exit