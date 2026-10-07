package modes

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/peggco/peggbot/internal/config"
	"github.com/peggco/peggbot/internal/github"
	"github.com/peggco/peggbot/internal/prompts"
	"github.com/peggco/peggbot/internal/runner"
)

func Run(ctx context.Context, cfg *config.Config, gh *github.Client) error {
	switch cfg.Mode {
	case "solve-issue":
		return SolveIssue(ctx, cfg, gh)
	case "review-pr":
		return ReviewPR(ctx, cfg, gh)
	case "respond":
		return Respond(ctx, cfg, gh)
	case "release-notes":
		return ReleaseNotes(ctx, cfg, gh)
	default:
		return fmt.Errorf("modes: unknown mode %q", cfg.Mode)
	}
}

func openEngine(ctx context.Context, cfg *config.Config, gh *github.Client, input string, files []string) (string, error) {
	filePrompt, err := cfg.LoadPromptFile()
	if err != nil {
		return "", err
	}
	systemPrompt := prompts.Compose(cfg.Mode, filePrompt)
	if strings.Contains(systemPrompt, "{actor}") {
		systemPrompt = strings.ReplaceAll(systemPrompt, "{actor}", cfg.Actor)
	}

	eng, err := runner.Open(ctx, cfg, gh)
	if err != nil {
		return "", err
	}
	defer eng.Close()

	return eng.Chat(ctx, cfg, systemPrompt, input, files)
}

func fail(ctx context.Context, cfg *config.Config, gh *github.Client, number int, err error) error {
	log.Printf("peggbot: mode %s failed: %v", cfg.Mode, err)
	if !cfg.DryRun && cfg.UnauthorizedAction == "comment" {
		_, _ = gh.PostComment(ctx, number, fmt.Sprintf(":warning: peggbot ran into a problem:\n\n```\n%v\n```", err))
	}
	return err
}
