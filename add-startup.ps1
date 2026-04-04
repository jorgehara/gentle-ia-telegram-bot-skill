# gentle-ia telegram-bot-skill - Agregar al Startup de Windows
$target = "C:\Users\JorgeHaraDevs\Desktop\go-telegram-opencode-bridge\start-stack.bat"
$shortcutPath = "$env:APPDATA\Microsoft\Windows\Start Menu\Programs\Startup\gentle-ia-telegram-bot-skill.lnk"

$shell = New-Object -ComObject WScript.Shell
$shortcut = $shell.CreateShortcut($shortcutPath)
$shortcut.TargetPath = $target
$shortcut.WorkingDirectory = "C:\Users\JorgeHaraDevs\Desktop\go-telegram-opencode-bridge"
$shortcut.Description = "gentle-ia telegram-bot-skill + OpenCode"
$shortcut.Save()

Write-Host "✅ Atajo creado en: $shortcutPath"
Write-Host ""
Write-Host "gentle-ia-telegram-bot-skill se iniciara automaticamente al arrancar Windows"