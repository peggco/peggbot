package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setEnv(t *testing.T, pairs map[string]string) {
	t.Helper()
	for k, v := range pairs {
		t.Setenv(k, v)
	}
}

func TestLoadValid(t *testing.T) {
	setEnv(t, map[string]string{
		"GITHUB_REPOSITORY":     "peggco/pegg",
		"GITHUB_ACTOR":          "octocat",
		"GITHUB_WORKSPACE":      "/workspace",
		"INPUT_MODE":            "solve-issue",
		"INPUT_TOKEN":           "tok",
		"INPUT_LLM_PROVIDER":    "anthropic",
		"INPUT_LLM_MODEL":       "claude-sonnet-4-5",
		"INPUT_LLM_API_KEY":     "sk-123",
		"INPUT_ISSUE_NUMBER":    "42",
		"INPUT_MAX_ITERATIONS":  "50",
		"INPUT_TIMEOUT_MINUTES": "15",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Owner != "peggco" || cfg.Repo != "pegg" {
		t.Errorf("owner/repo = %q/%q", cfg.Owner, cfg.Repo)
	}
	if cfg.IssueNumber != 42 {
		t.Errorf("issue number = %d", cfg.IssueNumber)
	}
	if cfg.MaxIterations != 50 {
		t.Errorf("max iterations = %d", cfg.MaxIterations)
	}
	if cfg.Timeout != 15*time.Minute {
		t.Errorf("timeout = %v", cfg.Timeout)
	}
	if cfg.RequiredPermission != "admin" {
		t.Errorf("default required permission = %q", cfg.RequiredPermission)
	}
	if cfg.Workspace != "/workspace" {
		t.Errorf("workspace = %q", cfg.Workspace)
	}
}

func TestLoadUnknownMode(t *testing.T) {
	setEnv(t, map[string]string{
		"GITHUB_REPOSITORY":  "o/r",
		"INPUT_MODE":         "explode",
		"INPUT_TOKEN":        "tok",
		"INPUT_LLM_PROVIDER": "anthropic",
		"INPUT_LLM_MODEL":    "m",
		"INPUT_LLM_API_KEY":  "k",
	})
	if _, err := Load(); err == nil {
		t.Fatal("expected error for unknown mode")
	}
}

func TestLoadMissingRepo(t *testing.T) {
	setEnv(t, map[string]string{
		"INPUT_MODE":         "respond",
		"INPUT_TOKEN":        "tok",
		"INPUT_LLM_PROVIDER": "anthropic",
		"INPUT_LLM_MODEL":    "m",
		"INPUT_LLM_API_KEY":  "k",
	})
	os.Unsetenv("GITHUB_REPOSITORY")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for missing repository")
	}
}

func TestPromptPath(t *testing.T) {
	setEnv(t, map[string]string{
		"GITHUB_REPOSITORY":  "o/r",
		"INPUT_MODE":         "respond",
		"INPUT_TOKEN":        "tok",
		"INPUT_LLM_PROVIDER": "anthropic",
		"INPUT_LLM_MODEL":    "m",
		"INPUT_LLM_API_KEY":  "k",
		"GITHUB_WORKSPACE":   "/ws",
		"INPUT_PROMPT_FILE":  ".github/prompts/respond.md",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := filepath.Join("/ws", ".github/prompts/respond.md")
	if got := cfg.PromptPath(); got != want {
		t.Errorf("prompt path = %q, want %q", got, want)
	}
}
