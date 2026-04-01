---
name: summarize-codebase
description: Generate a concise architectural summary of the current codebase
---

# Codebase Summarizer

Analyze the current codebase structure and generate a concise architectural overview.

## Steps

1. List the top-level directory structure with `ls` or `glob`
2. Read key entry-point files (main.go, index.ts, package.json, go.mod, etc.)
3. Identify the primary modules/packages and their responsibilities
4. Note major dependencies and external integrations
5. Identify the data flow (input → processing → output)

## Output Format

```markdown
# Codebase Summary — {PROJECT_NAME}

## Architecture Overview
{2-3 sentence high-level description}

## Key Modules
| Module | Responsibility |
|--------|---------------|
| ...    | ...           |

## Data Flow
{Mermaid diagram or text description}

## Key Dependencies
- ...

## Entry Points
- ...
```
