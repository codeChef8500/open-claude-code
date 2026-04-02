---
name: git-commit
description: Stage and commit changes with a conventional commit message
file_pattern: ".git"
tags: [git, commit, workflow]
---

Review the current git diff and staged changes, then create a well-formatted commit message following the Conventional Commits specification (https://www.conventionalcommits.org/).

Steps:
1. Run `git status` and `git diff --staged` to understand what is staged
2. If nothing is staged, run `git diff` to see unstaged changes and ask whether to stage them
3. Craft a commit message with:
   - Type: feat|fix|docs|style|refactor|perf|test|build|ci|chore
   - Optional scope in parentheses
   - Short imperative description (≤72 chars)
   - Optional body explaining *why* (not what)
   - Optional footer for breaking changes or issue references
4. Confirm with the user before committing
5. Run `git commit -m "<message>"`
