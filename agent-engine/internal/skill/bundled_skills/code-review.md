---
name: code-review
description: Perform a thorough code review of staged or recent changes
file_pattern: ".git"
tags: [git, review, quality]
---

Perform a structured code review of the changes provided (diff, file list, or PR description).

Review dimensions:
1. **Correctness** – Logic errors, off-by-one, null/nil dereferences, unhandled errors
2. **Security** – Injection vulnerabilities, hardcoded credentials, insecure defaults
3. **Performance** – Unnecessary allocations, N+1 queries, blocking calls in hot paths
4. **Readability** – Naming, comments, function length, cyclomatic complexity
5. **Tests** – Missing test coverage for new/changed behaviour
6. **API design** – Breaking changes, interface clarity, backward compatibility

Output format:
- Summary paragraph
- Numbered findings with severity: 🔴 Critical / 🟠 Major / 🟡 Minor / 💡 Suggestion
- Each finding: file:line, description, recommended fix
- Final verdict: Approve / Request Changes / Needs Discussion
