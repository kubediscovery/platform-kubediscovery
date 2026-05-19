# AGENTS

## Current reality (do not assume README plan is implemented)
- This repo is an early bootstrap, not the architecture described in `README.md`.
- Executable app code is only `main.go` in the module root.
- There are no internal packages, no CLI structure, no services, no tests, and no CI workflows yet.

## Source-of-truth priority in this repo
- Trust executable files first: `go.mod`, `main.go`, and runnable commands.
- Treat `README.md` as product/architecture intent (Portuguese), not current implementation state.

## Commands that actually reflect current behavior
- Run app: `go run .`
- Build check: `go build .`
- `go test ./...` currently fails because `go test` runs vet and flags `fmt.Println("Hello and welcome, %s!", s)` in `main.go`.
- If you need test-command signal before fixing code, use `go test -vet=off ./...` (there are no `_test.go` files right now).

## Repo boundaries that are easy to misread
- `.agents/skills/` and `skills-lock.json` are OpenCode skill assets/metadata, not application runtime code.
- Keep app changes focused on root Go module files unless new structure is explicitly introduced.
