# claude-statusline

Single-file Go program (`main.go`) that renders Claude Code's statusline. Reads JSON from stdin, outputs formatted text to stdout.

## Architecture

- **Input types** — Go structs matching Claude Code's statusline JSON schema
- **Segments** — functions with signature `func(*StatusInput) string`; return `""` to skip
- **`render()`** — joins non-empty segment outputs with a separator. Claude Code truncates the result if it exceeds terminal width
- **`osc8()`** — wraps text in OSC 8 hyperlink escape sequences

## Conventions

- Nerd Font icons are used for all indicators
- Hidden directories shorten to 2 chars (`.claude` → `.c`), regular dirs to 1 char
- Pointer types (`*struct`) for optional/nullable JSON fields

## CWD label

`cwdLabel` chooses the displayed text in this order:

1. `workspace.repo` present and `repo.owner == ghUser(repo.host)` → just `repo.name`
2. `workspace.repo` present → `owner/name`
3. Otherwise → `shortenPath(cwd)`

The OSC 8 hyperlink target stays `file://<dir>` in all cases. `ghUser` reads `~/.config/gh/hosts.yml` and returns the `user:` line under the matching host section, or `""` if unavailable.

## Model icons

`modelSegment` prepends mode glyphs before the model name. `effortIcon` returns up to two icons joined by a space:

- Thinking explicitly off (`thinking.enabled == false`) → `󰹏`, overrides effort
- Otherwise `effort.level` → `○` low, `◐` medium, `●` high, `◉` xhigh, `◈` max
- `fast_mode` → `↯`, independent and additive (e.g. `◉ ↯`)

## Release flow

Releases are tag-driven. Use lightweight tags (no `-a`/`-m`).

1. Push commits to `main`, then `git tag vX.Y.Z && git push origin vX.Y.Z`.
2. `.github/workflows/release.yml` runs GoReleaser and publishes binaries to GitHub Releases (used by `install.ps1` and `go install ...@vX.Y.Z`).
3. `.github/workflows/autoupdate.yml` opens a PR in this repo updating `flake.nix` to the new version.
4. Homebrew tap (`rileychh/homebrew-tap`) has its own daily `bump.yml` cron, or trigger it manually: `gh workflow run bump.yml -R rileychh/homebrew-tap -f formulae=cc-statusline`. It opens a PR; once CI is green, label it `pr-pull` to attach bottles and merge.
