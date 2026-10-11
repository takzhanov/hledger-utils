# AGENTS.md

## Scope
These instructions apply to the entire repository unless a deeper `AGENTS.md` overrides them.

## Working Style
- Keep changes small, focused, and easy to review.
- Prefer fixing root causes over adding one-off workarounds.
- Match the existing code style and project structure.
- Do not rename files, move modules, or refactor broadly unless the task requires it.
- Avoid introducing new dependencies unless clearly justified.

## Before Editing
- Inspect the relevant files and nearby patterns before making changes.
- Check for nested `AGENTS.md` files in subdirectories you touch.
- If requirements are ambiguous, choose the safest minimal implementation.

## File Changes
- Update only files directly related to the task.
- Keep comments concise and only add them when they improve clarity.
- Do not add license headers or unrelated cleanup.
- Preserve existing formatting conventions.

## Validation
- Prefer the smallest useful validation first, then broader checks if needed.
- Run targeted tests for the changed area when tests exist.
- Do not fix unrelated failing tests or unrelated lint issues.

## Git Hygiene
- Never commit, create branches, or rewrite history unless explicitly asked.
- Keep generated files and local artifacts out of the diff.

## Documentation
- Update docs when behavior, commands, or configuration changes.
- Keep README and usage examples aligned with the code.
