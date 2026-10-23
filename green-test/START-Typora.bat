@echo off
rem ============================================
rem  Typora Green - launcher
rem  First run: starts Typora AND creates shortcuts
rem  (package root + local desktop). Later runs: just start.
rem ============================================
setlocal
if not exist "%~dp0app\Typora.exe" (
  echo [ERROR] Typora.exe not found. Keep folder structure unchanged.
  pause
  exit /b 1
)

rem first run? marker lives OUTSIDE the package: a leftover shortcut inside the
rem package (left by whoever built the archive) must not skip shortcut creation.
if exist "%LOCALAPPDATA%\TyporaGreen.setup-done" goto RUN

set "PSCMD=$ws = New-Object -ComObject WScript.Shell; $lnk1 = $ws.CreateShortcut('%~dp0Typora Green.lnk'); $lnk1.TargetPath = '%~dp0app\Typora.exe'; $lnk1.WorkingDirectory = '%~dp0app'; $lnk1.IconLocation = '%~dp0app\Typora.exe,0'; $lnk1.Save(); $dk = (Get-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Explorer\User Shell Folders').Desktop; if ([string]::IsNullOrEmpty($dk)) { $dk = [Environment]::GetFolderPath('Desktop') }; $dk = [Environment]::ExpandEnvironmentVariables($dk); $lnk2 = $ws.CreateShortcut((Join-Path $dk 'Typora Green.lnk')); $lnk2.TargetPath = '%~dp0app\Typora.exe'; $lnk2.WorkingDirectory = '%~dp0app'; $lnk2.IconLocation = '%~dp0app\Typora.exe,0'; $lnk2.Save()"
powershell -NoProfile -Command "%PSCMD%"
if errorlevel 1 (
  echo [WARN] shortcut creation failed. Run SETUP.bat to create it later.
) else (
  echo First run: shortcuts created ^(package root + local desktop^).
  echo Next time just double-click the desktop icon.
)
echo done>"%LOCALAPPDATA%\TyporaGreen.setup-done"
ping -n 4 127.0.0.1 >nul

:RUN
start "" "%~dp0app\Typora.exe"
exit
