# cc-statusline

A Go program for [Claude Code's statusline](https://docs.anthropic.com/en/docs/claude-code/statusline) that displays session info in a compact, Nerd Font-styled format with clickable OSC 8 hyperlinks.

## Example output

```text
~/n/p/claude-statusline  · Sonnet 4.6 · 󱘲 27% · 󰓢 1.6k 27.9k · 󰊚 21% 18%
~/n/p/n/tattoo 󰘬 topic · Opus 4.6 · 󱘳 5% · 󰓢 1.6k 25.7k · 󰊚 15% 18%
~/n/p/n/t/.c/w/hey 󰌹 hey · Opus 4.6 · 󱘳 2% · 󰓢 0.3k 0.0k · 󰊚 16% 18%
```

## Segments

| Segment | Example | Description |
| ------- | ------- | ----------- |
| CWD | `~/n/p/claude-statusline` | Shortened path, clickable `file://` link |
| Git | `󰘬 topic` / `󰌹 hey` / `` | Branch, worktree name, or no-repo indicator |
| Model | `Opus 4.6` | Display name without parenthetical suffix |
| Context | `󱘲 27%` / `󱘳 5%` | Context window usage; 󱘳 for 1M+ windows |
| Tokens | `󰓢 1.6k 25.7k` | Cumulative input/output tokens |
| Rate limits | `󰊚 21% 18%` | 5h/7d usage, clickable link to usage settings |

Empty segments are omitted automatically.

## Requirements

- Go 1.21+
- A [Nerd Font](https://www.nerdfonts.com/) in your terminal
- A terminal with OSC 8 hyperlink support (Ghostty, iTerm2, Kitty, WezTerm)

## Install

```sh
go install github.com/rileychh/cc-statusline@latest
```

Or build from source:

```sh
git clone https://github.com/rileychh/cc-statusline
cd cc-statusline
go install .
```

## Configure

Add to `~/.claude/settings.json`:

```json
{
  "statusLine": {
    "type": "command",
    "command": "cc-statusline"
  }
}
```

The binary must be in your `PATH` (e.g. in `GOBIN`).

## Customization

Segments are composable functions with the signature `func(*StatusInput) string`. To add, remove, or reorder segments, edit the slice in `main()`:

```go
fmt.Print(render(&input, []segment{
    cwdSegment,
    modelSegment,
    contextSegment,
    tokensSegment,
    rateLimitsSegment,
}, " · "))
```

Return `""` from a segment to skip it.
