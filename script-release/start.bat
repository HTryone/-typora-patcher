@echo off
where node >nul 2>nul || (
    echo.
    echo  Node.js not found, please install: https://nodejs.org/zh-cn
    echo.
    pause
    exit /b 1
)
node "%~dp0typora_crack.js"
pause
