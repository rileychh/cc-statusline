package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"

	"golang.org/x/term"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

// Input types

type Model struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type ContextUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

type ContextWindow struct {
	TotalInputTokens    int           `json:"total_input_tokens"`
	TotalOutputTokens   int           `json:"total_output_tokens"`
	ContextWindowSize   int           `json:"context_window_size"`
	UsedPercentage      *float64      `json:"used_percentage"`
	RemainingPercentage *float64      `json:"remaining_percentage"`
	CurrentUsage        *ContextUsage `json:"current_usage"`
}

type Cost struct {
	TotalCostUSD       float64 `json:"total_cost_usd"`
	TotalDurationMS    int64   `json:"total_duration_ms"`
	TotalAPIDurationMS int64   `json:"total_api_duration_ms"`
	TotalLinesAdded    int     `json:"total_lines_added"`
	TotalLinesRemoved  int     `json:"total_lines_removed"`
}

type RateLimit struct {
	UsedPercentage float64 `json:"used_percentage"`
	ResetsAt       int64   `json:"resets_at"`
}

type RateLimits struct {
	FiveHour *RateLimit `json:"five_hour"`
	SevenDay *RateLimit `json:"seven_day"`
}

type Workspace struct {
	CurrentDir string `json:"current_dir"`
	ProjectDir string `json:"project_dir"`
}

type Worktree struct {
	Name           string `json:"name"`
	Path           string `json:"path"`
	Branch         string `json:"branch"`
	OriginalCWD    string `json:"original_cwd"`
	OriginalBranch string `json:"original_branch"`
}

type StatusInput struct {
	CWD            string        `json:"cwd"`
	SessionID      string        `json:"session_id"`
	TranscriptPath string        `json:"transcript_path"`
	Version        string        `json:"version"`
	Model          Model         `json:"model"`
	ContextWindow  ContextWindow `json:"context_window"`
	Cost           Cost          `json:"cost"`
	RateLimits     *RateLimits   `json:"rate_limits"`
	Workspace      Workspace     `json:"workspace"`
	Worktree       *Worktree     `json:"worktree"`
}

// Segment rendering

type segmentResult struct {
	text    string                         // rendered output (may contain OSC 8 escapes)
	display int                            // visible character count
	compact func(budget int) segmentResult // nil if not compactable
}

func seg(s string) segmentResult {
	return segmentResult{text: s, display: utf8.RuneCountInString(s)}
}

func seg8(text, display string) segmentResult {
	return segmentResult{text: text, display: utf8.RuneCountInString(display)}
}

type segment func(*StatusInput) segmentResult

func shortenPath(dir string) string {
	if home, err := os.UserHomeDir(); err == nil {
		if rel, err := filepath.Rel(home, dir); err == nil && !strings.HasPrefix(rel, "..") {
			dir = "~/" + rel
		}
	}
	parts := strings.Split(dir, string(filepath.Separator))
	for i := range parts[:max(len(parts)-1, 0)] {
		if len(parts[i]) > 1 && parts[i] != "~" {
			cut := 1
			if parts[i][0] == '.' {
				cut = 2
			}
			parts[i] = parts[i][:cut]
		}
	}
	return strings.Join(parts, "/")
}

func osc8(url, label string) string {
	return fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", url, label)
}

func gitBranch(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(bytes.TrimSpace(out))
}

func shortenBranch(branch string, budget int) string {
	if len(branch) <= budget {
		return branch
	}

	// Shorten prefix `/`-segments to 1 char
	parts := strings.Split(branch, "/")
	if len(parts) > 1 {
		for i := range parts[:len(parts)-1] {
			if len(parts[i]) > 1 {
				parts[i] = parts[i][:1]
			}
		}
		branch = strings.Join(parts, "/")
		if len(branch) <= budget {
			return branch
		}
	}

	// Truncate last segment at rightmost `-` or `_` that fits
	ellipsis := "…"
	last := parts[len(parts)-1]
	prefix := ""
	if len(parts) > 1 {
		prefix = strings.Join(parts[:len(parts)-1], "/") + "/"
	}
	remaining := budget - len(prefix) - utf8.RuneCountInString(ellipsis)
	if remaining > 0 && remaining < len(last) {
		cut := remaining
		for cut > 0 {
			if last[cut] == '-' || last[cut] == '_' {
				return prefix + last[:cut] + ellipsis
			}
			cut--
		}
	}

	// Hard truncate
	if remaining > 0 {
		return prefix + last[:remaining] + ellipsis
	}
	if budget > utf8.RuneCountInString(ellipsis) {
		return branch[:budget-utf8.RuneCountInString(ellipsis)] + ellipsis
	}
	return branch[:budget]
}

func cwdSegment(s *StatusInput) segmentResult {
	if s.Worktree != nil {
		origDir := s.Worktree.OriginalCWD
		cwdDisplay := shortenPath(origDir)
		cwd := osc8("file://"+origDir, cwdDisplay)
		wtDisplay := s.Worktree.Name
		wt := osc8("file://"+s.Worktree.Path, wtDisplay)
		display := cwdDisplay + " 󰌹 " + wtDisplay
		return seg8(cwd+" 󰌹 "+wt, display)
	}
	cwdDisplay := shortenPath(s.CWD)
	cwd := osc8("file://"+s.CWD, cwdDisplay)
	branch := gitBranch(s.CWD)
	if branch == "" {
		display := cwdDisplay + " "
		return seg8(cwd+" ", display)
	}
	if branch != "main" {
		display := cwdDisplay + " 󰘬 " + branch
		text := cwd + " 󰘬 " + branch
		r := seg8(text, display)
		r.compact = func(budget int) segmentResult {
			branchBudget := max(budget-utf8.RuneCountInString(cwdDisplay)-utf8.RuneCountInString(" 󰘬 "), 1)
			short := shortenBranch(branch, branchBudget)
			d := cwdDisplay + " 󰘬 " + short
			return seg8(cwd+" 󰘬 "+short, d)
		}
		return r
	}
	return seg8(cwd, cwdDisplay)
}

func modelSegment(s *StatusInput) segmentResult {
	name := s.Model.DisplayName
	if i := strings.Index(name, " ("); i != -1 {
		name = name[:i]
	}
	return seg(name)
}

func contextSegment(s *StatusInput) segmentResult {
	u := s.ContextWindow.CurrentUsage
	if u == nil {
		return segmentResult{}
	}
	current := u.InputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens
	if s.ContextWindow.ContextWindowSize == 0 {
		return segmentResult{}
	}
	pct := current * 100 / s.ContextWindow.ContextWindowSize
	icon := "󱘲"
	if s.ContextWindow.ContextWindowSize >= 1_000_000 {
		icon = "󱘳"
	}
	display := fmt.Sprintf("%s %d%%", icon, pct)
	return seg(display)
}

func tokensSegment(s *StatusInput) segmentResult {
	if s.ContextWindow.CurrentUsage == nil {
		return segmentResult{}
	}
	inK := float64(s.ContextWindow.TotalInputTokens) / 1000
	outK := float64(s.ContextWindow.TotalOutputTokens) / 1000
	display := fmt.Sprintf("󰓢 %.1fk %.1fk", inK, outK)
	return seg(display)
}

func rateLimitsSegment(s *StatusInput) segmentResult {
	if s.RateLimits == nil {
		return segmentResult{}
	}

	now := time.Now().Unix()

	// Check if either limit resets within 1 hour
	type countdown struct {
		icon      string
		remaining float64
		secsLeft  int64
	}
	var nearest *countdown

	if r := s.RateLimits.FiveHour; r != nil && r.ResetsAt > 0 {
		secsLeft := r.ResetsAt - now
		if secsLeft > 0 && secsLeft <= 3600 {
			nearest = &countdown{
				icon:      "󱑏",
				remaining: 100 - r.UsedPercentage,
				secsLeft:  secsLeft,
			}
		}
	}
	if r := s.RateLimits.SevenDay; r != nil && r.ResetsAt > 0 {
		secsLeft := r.ResetsAt - now
		if secsLeft > 0 && secsLeft <= 3600 {
			if nearest == nil || secsLeft < nearest.secsLeft {
				nearest = &countdown{
					icon:      "󱨴",
					remaining: 100 - r.UsedPercentage,
					secsLeft:  secsLeft,
				}
			}
		}
	}

	if nearest != nil {
		display := fmt.Sprintf("%s %.0f%% for %dm", nearest.icon, nearest.remaining, nearest.secsLeft/60)
		return seg8(osc8("https://claude.ai/settings/usage", display), display)
	}

	// Normal mode: show both percentages
	var parts []string
	if r := s.RateLimits.FiveHour; r != nil {
		parts = append(parts, fmt.Sprintf("%.0f%%", r.UsedPercentage))
	}
	if r := s.RateLimits.SevenDay; r != nil {
		parts = append(parts, fmt.Sprintf("%.0f%%", r.UsedPercentage))
	}
	display := "󰊚 " + strings.Join(parts, " ")
	return seg8(osc8("https://claude.ai/settings/usage", display), display)
}

func termWidth() int {
	tty, err := os.Open("/dev/tty")
	if err != nil {
		return 0
	}
	defer tty.Close()
	w, _, err := term.GetSize(int(tty.Fd()))
	if err != nil {
		return 0
	}
	return w
}

func maxDisplay() int {
	if w := termWidth(); w > 0 {
		return w - 4
	}
	return 76
}

// Assemble segments with separator, skipping empty ones.
// If total display width exceeds the terminal width, compact the longest compactable segment.
func render(s *StatusInput, segments []segment, sep string) string {
	var results []segmentResult
	for _, fn := range segments {
		if r := fn(s); r.display > 0 {
			results = append(results, r)
		}
	}
	if len(results) == 0 {
		return ""
	}

	sepWidth := utf8.RuneCountInString(sep) * (len(results) - 1)
	total := sepWidth
	for _, r := range results {
		total += r.display
	}

	limit := maxDisplay()
	if total > limit {
		// Find the longest compactable segment
		bestIdx := -1
		for i, r := range results {
			if r.compact != nil && (bestIdx == -1 || r.display > results[bestIdx].display) {
				bestIdx = i
			}
		}
		if bestIdx >= 0 {
			budget := max(results[bestIdx].display-(total-limit), 1)
			results[bestIdx] = results[bestIdx].compact(budget)
		}
	}

	var parts []string
	for _, r := range results {
		parts = append(parts, r.text)
	}
	return strings.Join(parts, sep)
}

func main() {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}

	var input StatusInput
	if err := json.Unmarshal(data, &input); err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Print(render(&input, []segment{
		cwdSegment,
		modelSegment,
		contextSegment,
		tokensSegment,
		rateLimitsSegment,
	}, " · "))
}
