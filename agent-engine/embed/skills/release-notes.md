---
name: release-notes
description: Generate a comprehensive release notes document from git history
---

# Release Notes Generator

Analyze the git commit history and generate comprehensive release notes.

## Steps

1. Run `git log --oneline --no-merges` to list commits since the last tag
2. Categorize commits into: Features, Bug Fixes, Performance, Breaking Changes, Documentation
3. For each category, write a clear user-facing description
4. Include migration guide if there are breaking changes
5. Format as Markdown with proper headings

## Output Format

```markdown
# Release Notes — v{VERSION}

## What's New
- ...

## Bug Fixes
- ...

## Breaking Changes
- ...

## Migration Guide
...
```
