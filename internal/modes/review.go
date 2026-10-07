package modes

import (
	"context"
	"fmt"
	"log"

	"github.com/peggco/peggbot/internal/config"
	"github.com/peggco/peggbot/internal/github"
)

func ReviewPR(ctx context.Context, cfg *config.Config, gh *github.Client) error {
	number := cfg.PullRequestNumber
	if number == 0 {
		return fmt.Errorf("review-pr: pull-request-number is required")
	}

	thread, err := gh.Thread(ctx, number)
	if err != nil {
		return err
	}

	input := fmt.Sprintf(
		"Review the pull request below.\n\n%s\n\n"+
			"Use github_get_pull_request and github_get_pr_diff for the full context, "+
			"then run /review and publish inline comments and a verdict.",
		thread,
	)

	out, err := openEngine(ctx, cfg, gh, input, nil)
	if err != nil {
		return fail(ctx, cfg, gh, number, err)
	}

	log.Printf("review-pr: agent finished for pull request #%d", number)
	if out != "" {
		log.Printf("review-pr: final output:\n%s", out)
	}
	return nil
}
