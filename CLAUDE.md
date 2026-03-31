# claude-statusline

Single-file Go program (`main.go`) that renders Claude Code's statusline. Reads JSON from stdin, outputs formatted text to stdout.

## Architecture

- **Input types** — Go structs matching Claude Code's statusline JSON schema
- **Segments** — functions with signature `func(*StatusInput) string`; return `""` to skip
- **`render()`** — joins non-empty segment outputs with a separator
- **`osc8()`** — wraps text in OSC 8 hyperlink escape sequences

## Conventions

- Nerd Font icons are used for all indicators
- Hidden directories shorten to 2 chars (`.claude` → `.c`), regular dirs to 1 char
- Pointer types (`*struct`) for optional/nullable JSON fields
- No external dependencies
