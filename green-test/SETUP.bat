@echo off
rem ============================================
rem  Typora Green - Setup (all-in-one menu)
rem  Double-click me and pick a number.
rem ============================================
setlocal enabledelayedexpansion
pushd "%~dp0app"
set "TS=%CD%\Typora.exe"
set "ICON=%CD%\resources\assets\file.ico"
set "UC=HKCU\Software\Classes"
popd

:MENU
cls
echo.
echo  ============================================
echo    Typora Green - Setup
echo  ============================================
echo.
echo    1. Create desktop icon          (recommended)
echo    2. Register .md file association + desktop icon
echo    3. Remove everything            (unregister + icons)
echo    4. Start Typora now
echo.
echo    0. Exit
echo.
set /p CHOICE=  Pick a number and press Enter: 

if "%CHOICE%"=="1" goto ICON
if "%CHOICE%"=="2" goto REG
if "%CHOICE%"=="3" goto UNREG
if "%CHOICE%"=="4" goto RUN
if "%CHOICE%"=="0" exit /b 0
goto MENU

:ICON
if not exist "%TS%" goto NOTFOUND
powershell -NoProfile -Command "$ws = New-Object -ComObject WScript.Shell; $dk = [Environment]::GetFolderPath('Desktop') + '\Typora Green.lnk'; $lnk = $ws.CreateShortcut($dk); $lnk.TargetPath = '%TS%'; $lnk.WorkingDirectory = '%~dp0app'; $lnk.IconLocation = '%ICON%'; $lnk.Save()"
echo.
echo  [OK] Desktop icon "Typora Green" created.
echo       Double-click it on your desktop to start Typora.
echo.
pause
goto MENU

:REG
if not exist "%TS%" goto NOTFOUND
pushd "%~dp0app"
reg add "%UC%\TyporaGreen.md" /ve /d "Typora Green" /f >nul
reg add "%UC%\TyporaGreen.md" /v "FriendlyTypeName" /d "Typora Green" /f >nul
reg add "%UC%\TyporaGreen.md\DefaultIcon" /ve /d "%ICON%" /f >nul
reg add "%UC%\TyporaGreen.md\shell\open\command" /ve /d "\"%TS%\" \"%%1\"" /f >nul
for %%E in (.md .markdown .mdown .mkd) do (
  reg add "%UC%\%%E\OpenWithProgids" /v "TyporaGreen.md" /d "" /f >nul 2>nul
)
set "PSCMD=$ws = New-Object -ComObject WScript.Shell; $sm = [Environment]::GetFolderPath('StartMenu') + '\Programs\Typora Green.lnk'; $dk = [Environment]::GetFolderPath('Desktop') + '\Typora Green.lnk'; $l1 = $ws.CreateShortcut($sm); $l1.TargetPath = '%TS%'; $l1.WorkingDirectory = '%CD%'; $l1.IconLocation = '%ICON%'; $l1.Save(); $l2 = $ws.CreateShortcut($dk); $l2.TargetPath = '%TS%'; $l2.WorkingDirectory = '%CD%'; $l2.IconLocation = '%ICON%'; $l2.Save()"
powershell -NoProfile -Command "%PSCMD%"
ie4uinit.exe -show >nul 2>nul
popd
echo.
echo  [OK] Registered. Shortcuts: Desktop + Start Menu.
echo.
echo  NEXT: a popup window is opening -
echo        1) pick "Typora Green"
echo        2) check "Always use this app"
echo        3) click OK
echo        Then double-clicking .md files opens Typora Green.
echo.
start "" rundll32.exe shell32.dll,OpenAs_RunDLL "%~dp0(ô.md"
pause
goto MENU

:UNREG
set "UC2=%UC%"
reg delete "%UC2%\TyporaGreen.md" /f >nul 2>nul
for %%E in (.md .markdown .mdown .mkd) do (
  reg delete "%UC2%\%%E\OpenWithProgids" /v "TyporaGreen.md" /f >nul 2>nul
)
del "%APPDATA%\Microsoft\Windows\Start Menu\Programs\Typora Green.lnk" 2>nul
echo removed>"%LOCALAPPDATA%\TyporaGreen.setup-done" 2>nul
powershell -NoProfile -Command "$dk=[Environment]::GetFolderPath('Desktop')+'\Typora Green.lnk'; if(Test-Path $dk){Remove-Item $dk}"
echo.
echo  [OK] Unregistered. Desktop + Start Menu icons removed.
echo.
pause
goto MENU

:RUN
if not exist "%TS%" goto NOTFOUND
start "" "%TS%"
goto MENU

:NOTFOUND
echo.
echo  [ERROR] Typora.exe not found. Keep the folder structure unchanged.
echo.
pause
goto MENU
