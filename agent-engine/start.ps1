<#
.SYNOPSIS
    一键启动 Agent Engine 全屏 TUI 主界面
.DESCRIPTION
    自动加载 .env → 编译 → 启动交互式全屏 TUI。
    支持快捷参数覆盖，例如：
      .\start.ps1                       # 默认启动
      .\start.ps1 -s                    # 跳过编译
      .\start.ps1 -m "gpt-4o"          # 指定模型
      .\start.ps1 -p "帮我写个hello"    # 单次提问
#>
param(
    [Alias("m")][string]$Model,
    [Alias("p")][string]$Prompt,
    [Alias("w")][string]$WorkDir,
    [Alias("s")][switch]$SkipBuild,
    [switch]$Verbose
)

$ErrorActionPreference = "Stop"
$Root = $PSScriptRoot
if (-not (Test-Path (Join-Path $Root "go.mod"))) {
    $Root = Split-Path -Parent $PSScriptRoot
}

# ── 1. Load .env ─────────────────────────────────────────────────────────────
$envFile = Join-Path $Root ".env"
if (Test-Path $envFile) {
    foreach ($line in (Get-Content $envFile -Encoding UTF8)) {
        $t = $line.Trim()
        if ($t -and -not $t.StartsWith("#")) {
            $eq = $t.IndexOf("=")
            if ($eq -gt 0) {
                $k = $t.Substring(0, $eq).Trim()
                $v = $t.Substring($eq + 1).Trim().Trim('"').Trim("'")
                [Environment]::SetEnvironmentVariable($k, $v, "Process")
            }
        }
    }
}

# ── 2. Resolve provider / model / key ────────────────────────────────────────
$provider = ($env:AGENT_ENGINE_PROVIDER, $env:LLM_PROVIDER, "openai" | Where-Object { $_ })[0]
$model    = if ($Model) { $Model }
            else { ($env:AGENT_ENGINE_MODEL, $env:LLM_MODEL, "MiniMax-M2.5" | Where-Object { $_ })[0] }
$apiKey   = ($env:AGENT_ENGINE_API_KEY, $env:ANTHROPIC_API_KEY, $env:OPENAI_API_KEY,
             $env:VLLM_API_KEY, $env:MINIMAX_API_KEY, $env:OPENROUTER_API_KEY | Where-Object { $_ })[0]
$baseURL  = ($env:AGENT_ENGINE_BASE_URL, $env:OPENAI_BASE_URL, $env:VLLM_BASE_URL | Where-Object { $_ })[0]

if (-not $apiKey) {
    Write-Host "`n  [ERROR] 未找到 API Key！请在 .env 中设置 AGENT_ENGINE_API_KEY" -ForegroundColor Red
    Write-Host "  支持: OPENAI_API_KEY / ANTHROPIC_API_KEY / MINIMAX_API_KEY / VLLM_API_KEY`n" -ForegroundColor Yellow
    exit 1
}

# ── 3. Display banner ────────────────────────────────────────────────────────
$masked = $apiKey.Substring(0, [Math]::Min(8, $apiKey.Length)) + "****"
Write-Host ""
Write-Host "  ╭──────────────────────────────────╮" -ForegroundColor Cyan
Write-Host "  │   Agent Engine  —  Interactive    │" -ForegroundColor Cyan
Write-Host "  ╰──────────────────────────────────╯" -ForegroundColor Cyan
Write-Host "  Provider : $provider" -ForegroundColor DarkCyan
Write-Host "  Model    : $model" -ForegroundColor DarkCyan
Write-Host "  API Key  : $masked" -ForegroundColor DarkCyan
if ($baseURL) { Write-Host "  Base URL : $baseURL" -ForegroundColor DarkCyan }
Write-Host ""

# ── 4. Go build ──────────────────────────────────────────────────────────────
$env:GOTMPDIR = "D:\tmp-go"
$env:GOCACHE  = "D:\tmp-go\cache"
if (-not (Test-Path $env:GOTMPDIR)) { New-Item -ItemType Directory -Path $env:GOTMPDIR -Force | Out-Null }

$bin = Join-Path $Root "agent-engine.exe"

if (-not $SkipBuild) {
    Write-Host "  [build] Compiling..." -ForegroundColor DarkGray -NoNewline
    Push-Location $Root
    try {
        & go build -o $bin ./cmd/agent-engine/ 2>&1 | Out-Null
        if ($LASTEXITCODE -ne 0) {
            Write-Host " FAILED" -ForegroundColor Red
            Write-Host "  Run 'go build ./...' for details." -ForegroundColor Yellow
            exit 1
        }
        Write-Host " OK" -ForegroundColor Green
    } finally { Pop-Location }
} else {
    if (-not (Test-Path $bin)) {
        Write-Host "  [ERROR] $bin 不存在，请去掉 -s 重新编译" -ForegroundColor Red; exit 1
    }
    Write-Host "  [build] Skipped" -ForegroundColor DarkGray
}

# ── 5. Export env ─────────────────────────────────────────────────────────────
$env:AGENT_ENGINE_PROVIDER = $provider
$env:AGENT_ENGINE_MODEL    = $model
$env:AGENT_ENGINE_API_KEY  = $apiKey
if ($baseURL) { $env:AGENT_ENGINE_BASE_URL = $baseURL }

# ── 6. Launch ─────────────────────────────────────────────────────────────────
$args_ = @()
if ($Model)   { $args_ += "--model";   $args_ += $model }
if ($Verbose) { $args_ += "--verbose" }
if ($WorkDir) { $args_ += "-C";        $args_ += $WorkDir }
if ($Prompt)  { $args_ += "-p";        $args_ += $Prompt }

if ($Prompt) {
    Write-Host "  [run] Single-shot: $Prompt" -ForegroundColor DarkGray
} else {
    Write-Host "  [run] Full-screen TUI  (Ctrl+C to quit)" -ForegroundColor DarkGray
}
Write-Host ""

& $bin @args_
exit $LASTEXITCODE
