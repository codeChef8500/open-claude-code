# Agent Engine — Go Rewrite

A complete Go rewrite of the Claude Code agentic engine core, extracted from the TypeScript implementation.

## Architecture

```
agent-engine/
├── cmd/agent-engine/          # HTTP server entry point
├── pkg/sdk/                   # Public Go SDK
├── internal/
│   ├── engine/                # Core query loop, types, context compaction
│   ├── provider/              # LLM provider adapters (Anthropic, OpenAI-compat)
│   ├── tool/                  # Tool interface, registry, orchestration
│   │   ├── bash/              # BashTool
│   │   ├── fileread/          # Read (file viewer)
│   │   ├── fileedit/          # Edit (find-and-replace)
│   │   ├── filewrite/         # Write (create/overwrite)
│   │   ├── grep/              # Grep (ripgrep wrapper)
│   │   ├── glob/              # Glob (doublestar)
│   │   ├── webfetch/          # WebFetch (HTML→Markdown)
│   │   ├── websearch/         # WebSearch
│   │   ├── askuser/           # AskUser
│   │   ├── todo/              # TodoWrite
│   │   ├── sendmessage/       # SendMessage
│   │   ├── sleep/             # Sleep
│   │   ├── taskstop/          # TaskStop
│   │   ├── notebookedit/      # NotebookEdit (.ipynb)
│   │   ├── brief/             # Brief (progress summary)
│   │   ├── cron/              # ScheduleCron
│   │   └── agentool/          # Task (sub-agent spawner)
│   ├── prompt/                # 6-layer system prompt assembly + cache
│   ├── permission/            # Permission checker + rules
│   ├── mode/                  # Undercover, AutoMode, FastMode, SideQuery
│   ├── skill/                 # Markdown skill loader
│   ├── plugin/                # hashicorp/go-plugin external tools
│   ├── buddy/                 # Companion system (Mulberry32 PRNG)
│   ├── memory/                # CLAUDE.md reader + LLM memory extractor
│   ├── session/               # JSONL transcript storage
│   ├── command/               # Slash command registry + built-ins
│   ├── agent/                 # Multi-agent coordinator
│   ├── daemon/                # Long-running background process (fsnotify)
│   ├── state/                 # AppState store + session state
│   ├── server/                # chi HTTP server + SSE streaming
│   └── util/                  # Errors, path, file, shell, cwd, format, env, …
└── embed/prompts/             # Embedded system prompt templates
```

## Quick Start

```bash
# Set your API key
export ANTHROPIC_API_KEY=sk-ant-...

# Build and run the HTTP server
make run

# The server listens on :8080 by default
```

## HTTP API

### Create a session
```bash
curl -X POST http://localhost:8080/api/v1/sessions \
  -H 'Content-Type: application/json' \
  -d '{"work_dir": "/path/to/project"}'
# → {"session_id":"<uuid>"}
```

### Send a message (non-streaming)
```bash
curl -X POST http://localhost:8080/api/v1/sessions/<id>/messages \
  -H 'Content-Type: application/json' \
  -d '{"text":"Write a hello world in Go"}'
```

### Send a message (SSE streaming)
```bash
curl -X POST http://localhost:8080/api/v1/sessions/<id>/messages \
  -H 'Content-Type: application/json' \
  -d '{"text":"Explain this codebase","stream":true}'
```

### Delete a session
```bash
curl -X DELETE http://localhost:8080/api/v1/sessions/<id>
```

## Go SDK

```go
import "github.com/wall-ai/agent-engine/pkg/sdk"

eng, err := sdk.New(
    sdk.WithWorkDir("/my/project"),
    sdk.WithAPIKey(os.Getenv("ANTHROPIC_API_KEY")),
    sdk.WithModel("claude-sonnet-4-5"),
)
if err != nil {
    log.Fatal(err)
}
defer eng.Close()

ctx := context.Background()
events := eng.SubmitMessage(ctx, "Refactor the auth module")
for ev := range events {
    if ev.Type == engine.EventTextDelta {
        fmt.Print(ev.Text)
    }
}
```

## Configuration

Settings are read from (highest → lowest precedence):
1. Environment variables prefixed `AGENT_ENGINE_` (e.g. `AGENT_ENGINE_MODEL`)
2. `~/.claude/config.json`
3. Built-in defaults

| Key | Default | Description |
|-----|---------|-------------|
| `provider` | `anthropic` | `anthropic` or `openai` |
| `model` | `claude-sonnet-4-5` | Model name |
| `max_tokens` | `8192` | Maximum output tokens |
| `thinking_budget` | `0` | Extended thinking token budget |
| `http_port` | `8080` | HTTP listen port |
| `verbose` | `false` | Enable debug logging |
| `auto_mode` | `false` | Enable Auto Mode (LLM classifier) |

## Development

```bash
make build    # Compile binary to ./bin/agent-engine
make test     # Run all tests
make lint     # golangci-lint
make fmt      # gofmt + goimports
```

## Phases Implemented

| Phase | Description | Status |
|-------|-------------|--------|
| 1 | Project scaffold (go.mod, Makefile, core types) | ✅ |
| 2 | Utility library (17 files) | ✅ |
| 3 | LLM provider adapters (Anthropic + OpenAI-compat) | ✅ |
| 4 | Core engine + state management | ✅ |
| 5 | System prompt + prompt cache (6-layer assembly) | ✅ |
| 6 | Tool system (registry, orchestration, permission) | ✅ |
| 7–9 | 17 tool implementations + cron + brief | ✅ |
| 10 | Mode system (undercover, auto, fast, side-query) | ✅ |
| 11 | Skills system (Markdown loader) | ✅ |
| 12 | Plugin system (hashicorp/go-plugin) | ✅ |
| 13 | Buddy system (Mulberry32 PRNG) | ✅ |
| 14 | Memory system (CLAUDE.md + LLM extractor) | ✅ |
| 15 | Session storage (JSONL transcript) | ✅ |
| 16 | Command system (slash commands) | ✅ |
| 17 | Multi-agent coordinator | ✅ |
| 18 | Daemon (fsnotify background process) | ✅ |
| 19 | Context compression (/compact) | ✅ |
| 20 | SDK + HTTP server | ✅ |
| 21 | Entry point + README | ✅ |

## License

MIT
