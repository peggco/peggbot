package modes

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/peggco/peggbot/internal/config"
	"github.com/peggco/peggbot/internal/github"
)

func Respond(ctx context.Context, cfg *config.Config, gh *github.Client) error {
	number := cfg.IssueNumber
	if number == 0 {
		number = cfg.PullRequestNumber
	}
	if number == 0 {
		return fmt.Errorf("respond: no issue or pull request number in event")
	}

	if cfg.Actor == cfg.BotLogin {
		log.Printf("respond: comment by the bot itself (%s), skipping", cfg.Actor)
		return nil
	}

	level, err := gh.ActorPermission(ctx, cfg.Actor)
	if err != nil {
		return err
	}
	if !github.HasPermission(level, cfg.RequiredPermission) {
		log.Printf("respond: %s has %q permission, need %q — ignoring", cfg.Actor, level, cfg.RequiredPermission)
		if !cfg.DryRun && cfg.UnauthorizedAction == "comment" {
			body := fmt.Sprintf(
				"@%s peggbot only responds to repository %s, and you have %q access here.",
				cfg.Actor, cfg.RequiredPermission, level)
			url, err := gh.PostComment(ctx, number, body)
			if err != nil {
				return fmt.Errorf("respond: post denial comment: %w", err)
			}
			log.Printf("respond: posted denial to #%d: %s", number, url)
		}
		return nil
	}

	thread, err := gh.Thread(ctx, number)
	if err != nil {
		return err
	}

	attachments, err := gh.DownloadAttachments(ctx, []string{thread}, cfg.MaxAttachments)
	if err != nil {
		log.Printf("respond: attachment download: %v", err)
	}

	input := fmt.Sprintf(
		"Below is the full conversation (the administrator's mention is one of the comments).\n\n%s\n%s",
		thread,
		github.RenderAttachmentListing(attachments),
	)

	out, err := openEngine(ctx, cfg, gh, input, attachments)
	if err != nil {
		return fail(ctx, cfg, gh, number, err)
	}
	if strings.TrimSpace(out) == "" {
		log.Printf("respond: agent produced no answer for #%d", number)
		return nil
	}

	if cfg.DryRun {
		log.Printf("respond: dry-run, not posting answer to #%d", number)
		log.Printf("respond: answer:\n%s", out)
		return nil
	}

	url, err := gh.PostComment(ctx, number, out)
	if err != nil {
		return fmt.Errorf("respond: post answer: %w", err)
	}
	log.Printf("respond: answered #%d at %s", number, url)
	return nil
}
