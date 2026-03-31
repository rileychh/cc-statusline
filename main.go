package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

type segment func(*StatusInput) string

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

func cwdSegment(s *StatusInput) string {
	if s.Worktree != nil {
		origDir := s.Worktree.OriginalCWD
		cwd := osc8("file://"+origDir, shortenPath(origDir))
		wt := osc8("file://"+s.Worktree.Path, s.Worktree.Name)
		return cwd + " 󰌹 " + wt
	}
	cwd := osc8("file://"+s.CWD, shortenPath(s.CWD))
	branch := gitBranch(s.CWD)
	if branch == "" {
		return cwd + " "
	}
	if branch != "main" {
		return cwd + " 󰘬 " + branch
	}
	return cwd
}

func modelSegment(s *StatusInput) string {
	name := s.Model.DisplayName
	if i := strings.Index(name, " ("); i != -1 {
		name = name[:i]
	}
	return name
}

func contextSegment(s *StatusInput) string {
	u := s.ContextWindow.CurrentUsage
	if u == nil {
		return ""
	}
	current := u.InputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens
	if s.ContextWindow.ContextWindowSize == 0 {
		return ""
	}
	pct := current * 100 / s.ContextWindow.ContextWindowSize
	icon := "󱘲"
	if s.ContextWindow.ContextWindowSize >= 1_000_000 {
		icon = "󱘳"
	}
	return fmt.Sprintf("%s %d%%", icon, pct)
}

func tokensSegment(s *StatusInput) string {
	if s.ContextWindow.CurrentUsage == nil {
		return ""
	}
	inK := float64(s.ContextWindow.TotalInputTokens) / 1000
	outK := float64(s.ContextWindow.TotalOutputTokens) / 1000
	return fmt.Sprintf("󰓢 %.1fk %.1fk", inK, outK)
}

func rateLimitsSegment(s *StatusInput) string {
	if s.RateLimits == nil {
		return ""
	}
	var parts []string
	if r := s.RateLimits.FiveHour; r != nil {
		parts = append(parts, fmt.Sprintf("%.0f%%", r.UsedPercentage))
	}
	if r := s.RateLimits.SevenDay; r != nil {
		parts = append(parts, fmt.Sprintf("%.0f%%", r.UsedPercentage))
	}
	return osc8("https://claude.ai/settings/usage", "󰊚 "+strings.Join(parts, " "))
}

// Assemble segments with separator, skipping empty ones.
func render(s *StatusInput, segments []segment, sep string) string {
	var parts []string
	for _, seg := range segments {
		if v := seg(s); v != "" {
			parts = append(parts, v)
		}
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
