<#
.SYNOPSIS
    Agent Engine CLI launcher — full-screen TUI or single-shot prompt.
.DESCRIPTION
    Loads .env config, builds, and launches agent-engine.
    Modes:
      1. Full-screen TUI (default) — Bubbletea alternate-screen UI
      2. Single prompt:  .\run-cli.ps1 -Prompt "hello"
      3. Verbose:        .\run-cli.ps1 -Verbose
.EXAMPLE
    .\scripts\run-cli.ps1
    .\scripts\run-cli.ps1 -Prompt "echo HELLO via Bash tool"
    .\scripts\run-cli.ps1 -Verbose -Model "gpt-4o"
#>

param(
    [string]$Prompt = "",
    [string]$Model = "",
    [string]$WorkDir = "",
    [string]$Resume = "",
    [string]$PermissionMode = "",
    [switch]$Verbose,
    [switch]$SkipBuild,
    [switch]$Help
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectRoot = Split-Path -Parent $ScriptDir

if ($Help) {
    Write-Host ""
    Write-Host "  Agent Engine CLI - Interactive E2E Test Launcher"
    Write-Host "  ================================================"
    Write-Host ""
    Write-Host "  Usage:"
    Write-Host "    .\scripts\run-cli.ps1                          # interactive REPL"
    Write-Host "    .\scripts\run-cli.ps1 -Prompt 'your question'  # single prompt"
    Write-Host "    .\scripts\run-cli.ps1 -Verbose                 # verbose logging"
    Write-Host "    .\scripts\run-cli.ps1 -Model 'gpt-4o'          # override model"
    Write-Host "    .\scripts\run-cli.ps1 -WorkDir 'C:\myproject'  # set work dir"
    Write-Host "    .\scripts\run-cli.ps1 -SkipBuild               # skip compilation"
    Write-Host "    .\scripts\run-cli.ps1 -Resume 'session-id'       # resume session"
    Write-Host "    .\scripts\run-cli.ps1 -PermissionMode 'auto'     # set perm mode"
    Write-Host ""
    Write-Host "  Environment (.env):"
    Write-Host "    Copy .env.example to .env and fill in your API key."
    Write-Host "    Supports AGENT_ENGINE_*, OPENAI_*, ANTHROPIC_*, VLLM_*, MINIMAX_*"
    Write-Host ""
    exit 0
}

# -- 1. Load .env file ----------------------------------------------------
$envFile = Join-Path $ProjectRoot ".env"
if (Test-Path $envFile) {
    Write-Host "[env] Loading $envFile" -ForegroundColor DarkGray
    foreach ($rawLine in (Get-Content -Path $envFile -Encoding UTF8)) {
        $line = $rawLine.Trim()
        if ($line -and -not $line.StartsWith("#")) {
            $eqIdx = $line.IndexOf("=")
            if ($eqIdx -gt 0) {
                $key = $line.Substring(0, $eqIdx).Trim()
                $val = $line.Substring($eqIdx + 1).Trim()
                # Strip surrounding quotes
                if ($val.Length -ge 2) {
                    $first = $val[0]; $last = $val[$val.Length - 1]
                    if (($first -eq [char]'"' -and $last -eq [char]'"') -or
                        ($first -eq [char]"'" -and $last -eq [char]"'")) {
                        $val = $val.Substring(1, $val.Length - 2)
                    }
                }
                [Environment]::SetEnvironmentVariable($key, $val, "Process")
            }
        }
    }
} else {
    Write-Host "[env] No .env file found, using system env vars" -ForegroundColor Yellow
    Write-Host "      Hint: copy .env.example to .env and fill in your API key" -ForegroundColor Yellow
}

# -- 2. Detect config -----------------------------------------------------
$provider = if ($env:AGENT_ENGINE_PROVIDER) { $env:AGENT_ENGINE_PROVIDER }
            elseif ($env:LLM_PROVIDER) { $env:LLM_PROVIDER }
            else { "openai" }

$detectedModel = if ($Model) { $Model }
                 elseif ($env:AGENT_ENGINE_MODEL) { $env:AGENT_ENGINE_MODEL }
                 elseif ($env:LLM_MODEL) { $env:LLM_MODEL }
                 else { "MiniMax-M2.5" }

$apiKey = if ($env:AGENT_ENGINE_API_KEY) { $env:AGENT_ENGINE_API_KEY }
          elseif ($env:ANTHROPIC_API_KEY) { $env:ANTHROPIC_API_KEY }
          elseif ($env:OPENAI_API_KEY) { $env:OPENAI_API_KEY }
          elseif ($env:VLLM_API_KEY) { $env:VLLM_API_KEY }
          elseif ($env:MINIMAX_API_KEY) { $env:MINIMAX_API_KEY }
          elseif ($env:OPENROUTER_API_KEY) { $env:OPENROUTER_API_KEY }
          else { "" }

$baseURL = if ($env:AGENT_ENGINE_BASE_URL) { $env:AGENT_ENGINE_BASE_URL }
           elseif ($env:OPENAI_BASE_URL) { $env:OPENAI_BASE_URL }
           elseif ($env:VLLM_BASE_URL) { $env:VLLM_BASE_URL }
           else { "" }

if (-not $apiKey) {
    Write-Host "" -ForegroundColor Red
    Write-Host "  [ERROR] No API key found!" -ForegroundColor Red
    Write-Host "  Set one of: AGENT_ENGINE_API_KEY / OPENAI_API_KEY / ANTHROPIC_API_KEY" -ForegroundColor Yellow
    Write-Host "  Or create a .env file (see .env.example)" -ForegroundColor Yellow
    Write-Host ""
    exit 1
}

$maskedKey = $apiKey.Substring(0, [Math]::Min(8, $apiKey.Length)) + "..."
Write-Host ""
Write-Host "  +-------------------------------------+" -ForegroundColor Cyan
Write-Host "  |  Agent Engine CLI - E2E Test        |" -ForegroundColor Cyan
Write-Host "  +-------------------------------------+" -ForegroundColor Cyan
Write-Host "  Provider : $provider" -ForegroundColor Green
Write-Host "  Model    : $detectedModel" -ForegroundColor Green
Write-Host "  API Key  : $maskedKey" -ForegroundColor Green
if ($baseURL) {
    Write-Host "  Base URL : $baseURL" -ForegroundColor Green
}
Write-Host ""

# -- 3. Go build environment ----------------------------------------------
$env:GOTMPDIR = "D:\tmp-go"
$env:GOCACHE  = "D:\tmp-go\cache"

if (-not (Test-Path $env:GOTMPDIR)) {
    New-Item -ItemType Directory -Path $env:GOTMPDIR -Force | Out-Null
}

# -- 4. Build --------------------------------------------------------------
$binPath = Join-Path $ProjectRoot "agent-engine.exe"

if (-not $SkipBuild) {
    Write-Host "[build] Compiling agent-engine..." -ForegroundColor DarkGray
    Push-Location $ProjectRoot
    try {
        $buildOutput = & go build -o $binPath ./cmd/agent-engine/ 2>&1
        if ($LASTEXITCODE -ne 0) {
            Write-Host "[build] FAILED" -ForegroundColor Red
            Write-Host $buildOutput -ForegroundColor Red
            exit 1
        }
        Write-Host "[build] OK: $binPath" -ForegroundColor DarkGray
    } finally {
        Pop-Location
    }
} else {
    if (-not (Test-Path $binPath)) {
        Write-Host "[ERROR] Binary not found: $binPath (remove -SkipBuild)" -ForegroundColor Red
        exit 1
    }
    Write-Host "[build] Skipped, using existing binary" -ForegroundColor DarkGray
}

# -- 5. Export env vars for the CLI ----------------------------------------
$env:AGENT_ENGINE_PROVIDER = $provider
$env:AGENT_ENGINE_MODEL    = $detectedModel
$env:AGENT_ENGINE_API_KEY  = $apiKey
if ($baseURL) {
    $env:AGENT_ENGINE_BASE_URL = $baseURL
}

# -- 6. Build CLI args -----------------------------------------------------
$cliArgs = @()

if ($Model) {
    $cliArgs += "--model"
    $cliArgs += $Model
}
if ($Verbose) {
    $cliArgs += "--verbose"
}
if ($WorkDir) {
    $cliArgs += "-C"
    $cliArgs += $WorkDir
}
if ($Prompt) {
    $cliArgs += "-p"
    $cliArgs += $Prompt
}
if ($Resume) {
    $cliArgs += "--resume"
    $cliArgs += $Resume
}
if ($PermissionMode) {
    $cliArgs += "--permission-mode"
    $cliArgs += $PermissionMode
}

# -- 7. Launch --------------------------------------------------------------
if ($Prompt) {
    Write-Host "[run] Single-shot: $Prompt" -ForegroundColor DarkGray
} else {
    Write-Host "[run] Full-screen TUI (Ctrl+C to quit, /help for commands)" -ForegroundColor DarkGray
}
Write-Host "==========================================" -ForegroundColor DarkGray
Write-Host ""

& $binPath @cliArgs
$exitCode = $LASTEXITCODE

Write-Host ""
Write-Host "==========================================" -ForegroundColor DarkGray
Write-Host "[done] agent-engine exited, code=$exitCode" -ForegroundColor DarkGray
exit $exitCode
