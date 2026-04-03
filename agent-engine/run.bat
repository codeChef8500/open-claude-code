@echo off
REM ── Agent Engine 快速启动 ──────────────────────────────────
REM 双击此文件即可启动全屏 TUI 交互模式
REM 首次使用请先将 .env.example 复制为 .env 并填入 API Key
REM ─────────────────────────────────────────────────────────────

cd /d "%~dp0"

REM 加载 .env
if exist .env (
    for /f "usebackq tokens=1,* delims==" %%A in (".env") do (
        set "line=%%A"
        if not "!line:~0,1!"=="#" (
            set "%%A=%%B"
        )
    )
)
setlocal enabledelayedexpansion

REM Go 编译环境
if not exist "D:\tmp-go" mkdir "D:\tmp-go"
set GOTMPDIR=D:\tmp-go
set GOCACHE=D:\tmp-go\cache

REM 编译
echo [build] Compiling agent-engine...
go build -o agent-engine.exe ./cmd/agent-engine/
if %errorlevel% neq 0 (
    echo [build] FAILED
    pause
    exit /b 1
)
echo [build] OK

REM 启动全屏 TUI
echo.
echo  ====== Agent Engine TUI ======
echo.
agent-engine.exe %*

pause
