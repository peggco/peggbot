# Solve issue — additional instructions

These instructions are appended to peggbot's default solve-issue prompt.
Customize them per repository.

- This repository uses a specific test command. Prefer:
  - `make test` for the full suite
  - `go test ./internal/...` for targeted checks
- Keep changes Go-idiomatic: `gofmt` before committing.
- Never touch `go.sum` unless you actually changed a dependency.
- Commit messages must follow conventional commits (e.g. `fix: ...`).
- If the issue mentions a version, verify it against `go.mod` first.