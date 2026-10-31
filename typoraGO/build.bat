@echo off
rem One-click build: double-click -> typoratool.exe (GUI subsystem, no CMD window)
rem Layout: src\main.go (entry)  src\ui  src\console  src\system  src\asar  src\fuses  src\patcher
rem         assets\ = icon + manifest      tools\make_icon.py = regenerate icon
cd /d "%~dp0"
where go >nul 2>nul || (
    echo [x] Go not found. Install it first: https://go.dev/dl/
    pause
    exit /b 1
)
set "RSRC=%USERPROFILE%\go\bin\rsrc.exe"
if not exist "src\rsrc.syso" (
    if exist "%RSRC%" (
        echo [i] generating src\rsrc.syso - icon and manifest ...
        "%RSRC%" -ico "assets\typoratool.ico" -manifest "assets\typoratool.exe.manifest" -o "src\rsrc.syso" -arch amd64
    ) else (
        echo [!] rsrc not found - building WITHOUT icon.
        echo     install once:  go install github.com/akavel/rsrc@latest
    )
)
go build -trimpath -ldflags "-s -w -H=windowsgui" -o typoratool.exe ./src || (
    echo [x] build failed
    pause
    exit /b 1
)
echo [ok] typoratool.exe built
pause
