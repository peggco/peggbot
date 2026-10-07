package modes

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/peggco/peggbot/internal/config"
	"github.com/peggco/peggbot/internal/github"

	githubv3 "github.com/google/go-github/v86/github"
)

func ReleaseNotes(ctx context.Context, cfg *config.Config, gh *github.Client) error {
	if cfg.ReleaseID == 0 {
		return fmt.Errorf("release-notes: release-id is required")
	}

	release, _, err := gh.Raw.Repositories.GetRelease(ctx, gh.Owner, gh.Repo, cfg.ReleaseID)
	if err != nil {
		return fmt.Errorf("release-notes: get release %d: %w", cfg.ReleaseID, err)
	}

	prevTag, err := previousReleaseTag(ctx, gh, release)
	if err != nil {
		return err
	}

	var commitsBlock string
	if prevTag != "" {
		cmp, _, err := gh.Raw.Repositories.CompareCommits(ctx, gh.Owner, gh.Repo, prevTag, release.GetTagName(), nil)
		if err != nil {
			return fmt.Errorf("release-notes: compare %s...%s: %w", prevTag, release.GetTagName(), err)
		}
		commitsBlock = renderCommits(cmp)
	} else {
		commitsBlock = "(no previous release found; the workspace git history is the fallback — inspect it with git if needed)"
	}

	input := fmt.Sprintf(
		"Rewrite the release notes for this release.\n\n"+
			"## Release\n- Tag: %s\n- Name: %s\n- Current body:\n%s\n\n"+
			"## Commits since previous release (%s)\n%s\n\n"+
			"Publish the new notes with github_update_release using release_id %d.",
		release.GetTagName(), release.GetName(), indent(release.GetBody()),
		displayTag(prevTag), commitsBlock, cfg.ReleaseID,
	)

	out, err := openEngine(ctx, cfg, gh, input, nil)
	if err != nil {
		return fail(ctx, cfg, gh, 0, err)
	}
	log.Printf("release-notes: agent finished for release %s", release.GetTagName())
	if out != "" {
		log.Printf("release-notes: final output:\n%s", out)
	}
	return nil
}

func previousReleaseTag(ctx context.Context, gh *github.Client, release *githubv3.RepositoryRelease) (string, error) {
	releases, _, err := gh.Raw.Repositories.ListReleases(ctx, gh.Owner, gh.Repo, &githubv3.ListOptions{PerPage: 50})
	if err != nil {
		return "", fmt.Errorf("release-notes: list releases: %w", err)
	}
	published := release.GetPublishedAt()
	for _, r := range releases {
		if r.GetID() == release.GetID() {
			continue
		}
		if r.GetDraft() {
			continue
		}
		if !published.IsZero() && r.GetPublishedAt().After(published.Time) {
			continue
		}
		return r.GetTagName(), nil
	}
	return "", nil
}

func renderCommits(cmp *githubv3.CommitsComparison) string {
	if len(cmp.Commits) == 0 {
		return "(no commits found)"
	}
	var b strings.Builder
	for _, c := range cmp.Commits {
		msg := ""
		if c.Commit != nil {
			msg = c.Commit.GetMessage()
			if i := strings.IndexByte(msg, '\n'); i >= 0 {
				msg = msg[:i]
			}
		}
		author := ""
		if c.Author != nil {
			author = c.Author.GetLogin()
		}
		fmt.Fprintf(&b, "- %s%s\n", msg, shortAuthor(author))
	}
	return strings.TrimSpace(b.String())
}

func shortAuthor(author string) string {
	if author == "" {
		return ""
	}
	return fmt.Sprintf(" (by %s)", author)
}

func displayTag(tag string) string {
	if tag == "" {
		return "none"
	}
	return tag
}

func indent(s string) string {
	if strings.TrimSpace(s) == "" {
		return "_(empty)_"
	}
	return "```\n" + strings.TrimSpace(s) + "\n```"
}
