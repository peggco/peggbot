package modes

import (
	"context"
	"fmt"
	"log"

	"github.com/peggco/peggbot/internal/config"
	"github.com/peggco/peggbot/internal/github"
)

func SolveIssue(ctx context.Context, cfg *config.Config, gh *github.Client) error {
	number := cfg.IssueNumber
	if number == 0 {
		return fmt.Errorf("solve-issue: issue-number is required")
	}

	thread, err := gh.Thread(ctx, number)
	if err != nil {
		return err
	}

	attachments, err := gh.DownloadAttachments(ctx, []string{thread}, cfg.MaxAttachments)
	if err != nil {
		log.Printf("solve-issue: attachment download: %v", err)
	}

	input := fmt.Sprintf(
		"Solve the following issue in the checked-out repository.\n\n%s\n%s\n\n"+
			"Branch to create: %s/issue-%d. Base branch: %s.",
		thread, github.RenderAttachmentListing(attachments),
		cfg.BranchPrefix, number, cfg.BaseBranch,
	)

	out, err := openEngine(ctx, cfg, gh, input, attachments)
	if err != nil {
		return fail(ctx, cfg, gh, number, err)
	}

	log.Printf("solve-issue: agent finished for issue #%d", number)
	if out != "" {
		log.Printf("solve-issue: final output:\n%s", out)
	}
	return nil
}
