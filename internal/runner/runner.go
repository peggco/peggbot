package runner

import (
	"context"
	"fmt"
	"os"

	"github.com/peggco/peggbot/internal/config"
	"github.com/peggco/peggbot/internal/github"
	"github.com/peggco/peggbot/internal/githubtools"

	"github.com/peggco/pegg/pkg/sdk"
)

var BaseTools = []string{
	"read", "edit", "write", "list", "glob", "grep",
	"bash",
	"webfetch", "websearch",
	"task", "taskstatus",
	"todoread", "todowrite",
	"loadskill",
	"enterplanmode", "exitplanmode",
}

func GitHubToolNames(tools []sdk.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		names = append(names, t.Name())
	}
	return names
}

type Engine struct {
	SDK   *sdk.Engine
	Tools []sdk.Tool
}

func Open(ctx context.Context, cfg *config.Config, gh *github.Client) (*Engine, error) {
	eng, err := sdk.Open(ctx, sdk.Options{
		APIKeys:         map[string]string{cfg.Provider: cfg.APIKey},
		Providers:       []sdk.LLMConfig{{Provider: cfg.Provider, APIKey: cfg.APIKey, BaseURL: cfg.BaseURL}},
		Workspace:       cfg.Workspace,
		DisableRecorder: true,
	})
	if err != nil {
		return nil, fmt.Errorf("runner: open engine: %w", err)
	}

	tools, err := githubtools.NewGitHubTools(githubtools.GitHubToolOptions{
		Token:   cfg.Token,
		Owner:   cfg.Owner,
		Repo:    cfg.Repo,
		BaseURL: cfg.GHESBaseURL,
	})
	if err != nil {
		_ = eng.Close()
		return nil, fmt.Errorf("runner: github tools: %w", err)
	}
	for _, t := range tools {
		if err := eng.RegisterTool(t); err != nil {
			_ = eng.Close()
			return nil, fmt.Errorf("runner: register tool %q: %w", t.Name(), err)
		}
	}

	if dir := cfg.SkillsPath(); dir != "" {
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			if err := eng.LoadSkills(dir); err != nil {
				_ = eng.Close()
				return nil, fmt.Errorf("runner: load skills from %s: %w", dir, err)
			}
		}
	}

	return &Engine{SDK: eng, Tools: tools}, nil
}

func (e *Engine) Close() error {
	return e.SDK.Close()
}

func (e *Engine) Chat(ctx context.Context, cfg *config.Config, systemPrompt, input string, files []string) (string, error) {
	req := sdk.ChatRequest{
		Input:         input,
		Provider:      cfg.Provider,
		Model:         cfg.Model,
		SystemPrompt:  systemPrompt,
		Tools:         append(append([]string{}, BaseTools...), GitHubToolNames(e.Tools)...),
		MaxIterations: cfg.MaxIterations,
		Timeout:       cfg.Timeout,
		Files:         files,
	}

	resp, err := e.SDK.ChatStream(ctx, req, func(ev sdk.ChatEvent) error {
		switch ev.Type {
		case sdk.EventToken:
			fmt.Fprint(os.Stdout, ev.Content)
		case sdk.EventReasoning:
			fmt.Fprintf(os.Stdout, "\n[reasoning] %s\n", ev.Reasoning)
		case sdk.EventToolCall:
			fmt.Fprintf(os.Stdout, "\n[tool] %s(%s)\n", ev.ToolCall.Function.Name, ev.ToolCall.Function.Arguments)
		case sdk.EventToolResult:
			fmt.Fprintf(os.Stdout, "[result] %s\n", truncate(ev.ToolOutput, 500))
		case sdk.EventDone:
			fmt.Fprintf(os.Stdout, "\n[done] iterations=%d usage=%+v\n", ev.Messages, ev.Tokens)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("runner: chat: %w", err)
	}
	return resp.Output, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
