package github

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	githubv3 "github.com/google/go-github/v86/github"
)

var markdownImageRE = regexp.MustCompile(`!\[[^\]]*\]\((https?://[^)\s]+)\)`)

func (c *Client) DownloadAttachments(ctx context.Context, bodies []string, max int) ([]string, error) {
	seen := map[string]bool{}
	var urls []string
	for _, body := range bodies {
		for _, m := range markdownImageRE.FindAllStringSubmatch(body, -1) {
			if len(m) < 2 || seen[m[1]] {
				continue
			}
			seen[m[1]] = true
			urls = append(urls, m[1])
		}
	}
	if len(urls) > max {
		urls = urls[:max]
	}

	var files []string
	for _, url := range urls {
		path, err := c.download(ctx, url)
		if err != nil {
			fmt.Fprintf(os.Stderr, "peggbot: attachment download failed for %s: %v\n", url, err)
			continue
		}
		files = append(files, path)
	}
	return files, nil
}

func (c *Client) download(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	req.Header.Set("Accept", "image/*")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}
	if resp.ContentLength > 10<<20 {
		return "", fmt.Errorf("attachment too large (%d bytes)", resp.ContentLength)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20+1))
	if err != nil {
		return "", err
	}
	if len(data) > 10<<20 {
		return "", fmt.Errorf("attachment too large")
	}
	if ct := resp.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(ct, "image/") {
		return "", fmt.Errorf("attachment is not an image (%s)", ct)
	}
	dir, err := os.MkdirTemp("", "peggbot-attachments")
	if err != nil {
		return "", err
	}
	ext := extFor(resp.Header.Get("Content-Type"))
	name := fmt.Sprintf("attachment-%d%s", len(data), ext)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func extFor(contentType string) string {
	switch strings.ToLower(contentType) {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/bmp":
		return ".bmp"
	case "image/svg+xml":
		return ".svg"
	default:
		return ".img"
	}
}

func (c *Client) PostComment(ctx context.Context, number int, body string) (string, error) {
	if strings.TrimSpace(body) == "" {
		return "", fmt.Errorf("github: refusing to post an empty comment")
	}
	comment, _, err := c.Raw.Issues.CreateComment(ctx, c.Owner, c.Repo, number, &githubv3.IssueComment{Body: githubv3.String(body)})
	if err != nil {
		return "", fmt.Errorf("github: post comment on #%d: %w", number, err)
	}
	return comment.GetHTMLURL(), nil
}

func (c *Client) Thread(ctx context.Context, number int) (string, error) {
	issue, _, err := c.Raw.Issues.Get(ctx, c.Owner, c.Repo, number)
	if err != nil {
		return "", fmt.Errorf("github: get issue/PR #%d: %w", number, err)
	}
	comments, _, err := c.Raw.Issues.ListComments(ctx, c.Owner, c.Repo, number, &githubv3.IssueListCommentsOptions{})
	if err != nil {
		return "", fmt.Errorf("github: list comments on #%d: %w", number, err)
	}

	var b bytes.Buffer
	fmt.Fprintf(&b, "### %s #%d: %s\n", kindOf(issue), number, issue.GetTitle())
	fmt.Fprintf(&b, "State: %s | Author: %s\n\n", issue.GetState(), loginOf(issue.GetUser()))
	if body := issue.GetBody(); body != "" {
		fmt.Fprintf(&b, "%s\n\n", body)
	}
	if len(comments) > 0 {
		b.WriteString("---\n\n**Thread comments:**\n\n")
		for _, cm := range comments {
			fmt.Fprintf(&b, "**%s** (%s):\n", loginOf(cm.GetUser()), cm.GetCreatedAt().Format("2006-01-02 15:04"))
			if txt := cm.GetBody(); txt != "" {
				fmt.Fprintf(&b, "%s\n", txt)
			} else {
				b.WriteString("_(empty comment)_\n")
			}
			b.WriteString("\n")
		}
	}
	return b.String(), nil
}

func kindOf(issue *githubv3.Issue) string {
	if issue.IsPullRequest() {
		return "Pull Request"
	}
	return "Issue"
}

func loginOf(u *githubv3.User) string {
	if u == nil {
		return "unknown"
	}
	return u.GetLogin()
}

func RenderAttachmentListing(files []string) string {
	if len(files) == 0 {
		return ""
	}
	var b bytes.Buffer
	b.WriteString("\nImage attachments from the thread (downloaded and available to you):\n")
	for _, f := range files {
		fmt.Fprintf(&b, "- %s\n", filepath.Base(f))
	}
	return b.String()
}
