package githubtools

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	githubv3 "github.com/google/go-github/v86/github"

	"github.com/peggco/pegg/pkg/sdk"
)

var markdownImageRE = regexp.MustCompile(`!\[[^\]]*\]\((https?://[^)\s]+)\)`)

type GitHubToolOptions struct {
	Token     string
	Owner     string
	Repo      string
	BaseURL   string
	UploadURL string
}

func NewGitHubTools(opts GitHubToolOptions) ([]sdk.Tool, error) {
	if opts.Token == "" {
		return nil, fmt.Errorf("githubtools: token is required")
	}
	if opts.Owner == "" || opts.Repo == "" {
		return nil, fmt.Errorf("githubtools: owner and repo are required")
	}

	var client *githubv3.Client
	if opts.BaseURL != "" {
		var err error
		client, err = githubv3.NewEnterpriseClient(opts.BaseURL, opts.UploadURL, nil)
		if err != nil {
			return nil, fmt.Errorf("githubtools: new enterprise client: %w", err)
		}
	} else {
		client = githubv3.NewClient(nil)
	}
	client = client.WithAuthToken(opts.Token)

	kit := &githubToolkit{client: client, owner: opts.Owner, repo: opts.Repo}
	tools := []sdk.Tool{
		spec("github_get_issue",
			"Fetch a GitHub issue with its full conversation: title, body, state, labels, author and every comment. Returns JSON. Use this before working on an issue.",
			obj(map[string]any{
				"issue_number": intProp("The issue number, e.g. 42."),
			}, []string{"issue_number"}),
			kit.getIssue),
		spec("github_get_pull_request",
			"Fetch a GitHub pull request with its files, conversation comments, review comments and reviews. Returns JSON. Use this to understand a PR before reviewing it.",
			obj(map[string]any{
				"pull_number": intProp("The pull request number, e.g. 7."),
			}, []string{"pull_number"}),
			kit.getPullRequest),
		spec("github_get_pr_diff",
			"Fetch the unified diff of a GitHub pull request as raw text. Use this to review the exact changes a PR makes.",
			obj(map[string]any{
				"pull_number": intProp("The pull request number, e.g. 7."),
			}, []string{"pull_number"}),
			kit.getPullRequestDiff),
		spec("github_post_issue_comment",
			"Post a comment on a GitHub issue or pull request conversation. Use this to report progress, ask questions or answer the reporter.",
			obj(map[string]any{
				"issue_number": intProp("The issue or pull request number."),
				"body":         strProp("The comment body in Markdown."),
			}, []string{"issue_number", "body"}),
			kit.postIssueComment),
		spec("github_update_issue",
			"Update a GitHub issue or pull request: change its title, body or state. state is 'open' or 'closed'. Only pass the fields you want to change.",
			obj(map[string]any{
				"issue_number": intProp("The issue or pull request number."),
				"title":        strProp("New title (optional)."),
				"body":         strProp("New body in Markdown (optional)."),
				"state":        strProp("New state: 'open' or 'closed' (optional)."),
			}, []string{"issue_number"}),
			kit.updateIssue),
		spec("github_add_labels",
			"Add labels to a GitHub issue or pull request.",
			obj(map[string]any{
				"issue_number": intProp("The issue or pull request number."),
				"labels":       arrProp("Labels to add, e.g. [\"peggbot\", \"wip\"]."),
			}, []string{"issue_number", "labels"}),
			kit.addLabels),
		spec("github_remove_label",
			"Remove a label from a GitHub issue or pull request.",
			obj(map[string]any{
				"issue_number": intProp("The issue or pull request number."),
				"label":        strProp("The label to remove."),
			}, []string{"issue_number", "label"}),
			kit.removeLabel),
		spec("github_create_pull_request",
			"Create a GitHub pull request. The head branch must already exist in the repository (push it with git first).",
			obj(map[string]any{
				"title": strProp("The pull request title."),
				"head":  strProp("The branch with your changes, e.g. 'peggbot/issue-42'."),
				"base":  strProp("The branch to merge into, usually 'main'."),
				"body":  strProp("The pull request body in Markdown. Reference the issue with 'Closes #42' when it fixes one."),
				"draft": boolProp("Create as a draft pull request (optional, default false)."),
			}, []string{"title", "head", "base"}),
			kit.createPullRequest),
		spec("github_create_pr_review_comment",
			"Create an inline review comment on a specific line of a file in a GitHub pull request. Use this to leave line-level feedback during a review.",
			obj(map[string]any{
				"pull_number": intProp("The pull request number."),
				"body":        strProp("The comment body in Markdown."),
				"path":        strProp("The file path relative to the repository root, e.g. 'src/main.go'."),
				"line":        intProp("The line number in the diff the comment applies to."),
				"commit_id":   strProp("Optional commit SHA to anchor the comment (defaults to the PR head)."),
				"side":        strProp("Optional diff side: 'LEFT' or 'RIGHT' (default 'RIGHT')."),
			}, []string{"pull_number", "body", "path", "line"}),
			kit.createPrReviewComment),
		spec("github_submit_review",
			"Submit a review on a GitHub pull request with an overall verdict. event is 'APPROVE', 'REQUEST_CHANGES' or 'COMMENT'. Optionally attaches inline comments.",
			obj(map[string]any{
				"pull_number": intProp("The pull request number."),
				"body":        strProp("The review summary in Markdown."),
				"event":       strProp("The review verdict: 'APPROVE', 'REQUEST_CHANGES' or 'COMMENT'."),
				"comments":    arrProp("Optional inline comments: [{\"path\": \"src/main.go\", \"line\": 12, \"body\": \"...\"}]"),
			}, []string{"pull_number", "event"}),
			kit.submitReview),
		spec("github_list_pr_reviews",
			"List all reviews already submitted on a GitHub pull request.",
			obj(map[string]any{
				"pull_number": intProp("The pull request number."),
			}, []string{"pull_number"}),
			kit.listPrReviews),
		spec("github_get_actor_permission",
			"Check the permission level a GitHub user has on the repository. Returns 'admin', 'write', 'read' or 'none'. Use this to gate privileged actions.",
			obj(map[string]any{
				"username": strProp("The GitHub login to check, e.g. 'octocat'."),
			}, []string{"username"}),
			kit.getActorPermission),
		spec("github_get_release",
			"Fetch a GitHub release by its numeric id. Returns JSON with the tag, name, body and assets.",
			obj(map[string]any{
				"release_id": intProp("The release id, e.g. 12345678."),
			}, []string{"release_id"}),
			kit.getRelease),
		spec("github_get_release_by_tag",
			"Fetch a GitHub release by its tag name, e.g. 'v0.1.3'.",
			obj(map[string]any{
				"tag": strProp("The release tag name."),
			}, []string{"tag"}),
			kit.getReleaseByTag),
		spec("github_update_release",
			"Update the body (notes) of an existing GitHub release. Use this to publish generated release notes.",
			obj(map[string]any{
				"release_id": intProp("The release id to update."),
				"body":       strProp("The new release body in Markdown."),
			}, []string{"release_id", "body"}),
			kit.updateRelease),
		spec("github_list_releases",
			"List the most recent GitHub releases of the repository, newest first.",
			obj(map[string]any{
				"per_page": intProp("How many releases to return (optional, default 10)."),
			}, []string{}),
			kit.listReleases),
		spec("github_compare_tags",
			"Fetch the list of commits between two git refs or tags, e.g. base 'v0.1.2' head 'v0.1.3'. Use this to build release notes from commit messages.",
			obj(map[string]any{
				"base": strProp("The base ref or tag, e.g. 'v0.1.2'."),
				"head": strProp("The head ref or tag, e.g. 'v0.1.3'."),
			}, []string{"base", "head"}),
			kit.compareTags),
	}
	return tools, nil
}

type githubToolkit struct {
	client *githubv3.Client
	owner  string
	repo   string
}

func spec(name, description string, parameters any, fn func(ctx context.Context, args string) (string, error)) sdk.Tool {
	return sdk.NewTool(name, description, parameters, fn)
}

func obj(properties map[string]any, required []string) map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}
}

func strProp(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func intProp(description string) map[string]any {
	return map[string]any{"type": "integer", "description": description}
}

func boolProp(description string) map[string]any {
	return map[string]any{"type": "boolean", "description": description}
}

func arrProp(description string) map[string]any {
	return map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": description}
}

func (k *githubToolkit) json(v any) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("{\"error\": %q}", err.Error())
	}
	return string(data)
}

func (k *githubToolkit) getIssue(ctx context.Context, args string) (string, error) {
	var p struct {
		IssueNumber int `json:"issue_number"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("github_get_issue: %w", err)
	}
	issue, _, err := k.client.Issues.Get(ctx, k.owner, k.repo, p.IssueNumber)
	if err != nil {
		return "", fmt.Errorf("github_get_issue: %w", err)
	}
	comments, _, err := k.client.Issues.ListComments(ctx, k.owner, k.repo, p.IssueNumber, nil)
	if err != nil {
		return "", fmt.Errorf("github_get_issue: list comments: %w", err)
	}
	labels := make([]string, 0, len(issue.Labels))
	for _, l := range issue.Labels {
		if l.Name != nil {
			labels = append(labels, *l.Name)
		}
	}
	sort.Strings(labels)

	type commentView struct {
		Author      string   `json:"author"`
		AuthorType  string   `json:"author_type"`
		Body        string   `json:"body"`
		Attachments []string `json:"attachments,omitempty"`
		CreatedAt   string   `json:"created_at,omitempty"`
	}
	thread := make([]commentView, 0, len(comments))
	seen := map[string]bool{}
	for _, c := range comments {
		v := commentView{
			Author:     authorOf(c.GetUser()),
			AuthorType: authorAssociation(c.GetAuthorAssociation()),
			Body:       c.GetBody(),
			CreatedAt:  c.GetCreatedAt().String(),
		}
		v.Attachments = imageURLs(c.GetBody(), seen)
		thread = append(thread, v)
	}

	out := map[string]any{
		"number":      p.IssueNumber,
		"title":       issue.GetTitle(),
		"body":        issue.GetBody(),
		"state":       issue.GetState(),
		"author":      authorOf(issue.GetUser()),
		"author_type": authorAssociation(issue.GetAuthorAssociation()),
		"labels":      labels,
		"created_at":  issue.GetCreatedAt().String(),
		"updated_at":  issue.GetUpdatedAt().String(),
		"html_url":    issue.GetHTMLURL(),
		"attachments": imageURLs(issue.GetBody(), seen),
		"comments":    thread,
	}
	return k.json(out), nil
}

func (k *githubToolkit) getPullRequest(ctx context.Context, args string) (string, error) {
	var p struct {
		PullNumber int `json:"pull_number"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("github_get_pull_request: %w", err)
	}
	pr, _, err := k.client.PullRequests.Get(ctx, k.owner, k.repo, p.PullNumber)
	if err != nil {
		return "", fmt.Errorf("github_get_pull_request: %w", err)
	}
	files, _, err := k.client.PullRequests.ListFiles(ctx, k.owner, k.repo, p.PullNumber, nil)
	if err != nil {
		return "", fmt.Errorf("github_get_pull_request: list files: %w", err)
	}
	convComments, _, err := k.client.Issues.ListComments(ctx, k.owner, k.repo, p.PullNumber, nil)
	if err != nil {
		return "", fmt.Errorf("github_get_pull_request: list comments: %w", err)
	}
	reviewComments, _, err := k.client.PullRequests.ListComments(ctx, k.owner, k.repo, p.PullNumber, nil)
	if err != nil {
		return "", fmt.Errorf("github_get_pull_request: list review comments: %w", err)
	}
	reviews, _, err := k.client.PullRequests.ListReviews(ctx, k.owner, k.repo, p.PullNumber, nil)
	if err != nil {
		return "", fmt.Errorf("github_get_pull_request: list reviews: %w", err)
	}

	type fileView struct {
		Filename  string `json:"filename"`
		Additions int    `json:"additions"`
		Deletions int    `json:"deletions"`
		Changes   int    `json:"changes"`
		Status    string `json:"status"`
		Patch     string `json:"patch,omitempty"`
	}
	fileViews := make([]fileView, 0, len(files))
	for _, f := range files {
		fileViews = append(fileViews, fileView{
			Filename:  f.GetFilename(),
			Additions: f.GetAdditions(),
			Deletions: f.GetDeletions(),
			Changes:   f.GetChanges(),
			Status:    f.GetStatus(),
			Patch:     f.GetPatch(),
		})
	}

	type commentView struct {
		Author     string `json:"author"`
		AuthorType string `json:"author_type"`
		Body       string `json:"body"`
		Path       string `json:"path,omitempty"`
		Line       int    `json:"line,omitempty"`
	}
	conv := make([]commentView, 0, len(convComments))
	for _, c := range convComments {
		conv = append(conv, commentView{Author: authorOf(c.GetUser()), AuthorType: authorAssociation(c.GetAuthorAssociation()), Body: c.GetBody()})
	}
	inline := make([]commentView, 0, len(reviewComments))
	for _, c := range reviewComments {
		inline = append(inline, commentView{Author: authorOf(c.GetUser()), AuthorType: authorAssociation(c.GetAuthorAssociation()), Body: c.GetBody(), Path: c.GetPath(), Line: c.GetLine()})
	}
	reviewViews := make([]commentView, 0, len(reviews))
	for _, r := range reviews {
		reviewViews = append(reviewViews, commentView{Author: authorOf(r.GetUser()), AuthorType: authorAssociation(r.GetAuthorAssociation()), Body: r.GetBody()})
	}

	labels := make([]string, 0, len(pr.Labels))
	for _, l := range pr.Labels {
		if l.Name != nil {
			labels = append(labels, *l.Name)
		}
	}
	sort.Strings(labels)

	out := map[string]any{
		"number":          p.PullNumber,
		"title":           pr.GetTitle(),
		"body":            pr.GetBody(),
		"state":           pr.GetState(),
		"merged":          pr.GetMerged(),
		"draft":           pr.GetDraft(),
		"author":          authorOf(pr.GetUser()),
		"author_type":     authorAssociation(pr.GetAuthorAssociation()),
		"head":            pr.GetHead().GetRef(),
		"head_sha":        pr.GetHead().GetSHA(),
		"base":            pr.GetBase().GetRef(),
		"labels":          labels,
		"additions":       pr.GetAdditions(),
		"deletions":       pr.GetDeletions(),
		"changed_files":   pr.GetChangedFiles(),
		"commits":         pr.GetCommits(),
		"html_url":        pr.GetHTMLURL(),
		"files":           fileViews,
		"conversation":    conv,
		"review_comments": inline,
		"reviews":         reviewViews,
	}
	return k.json(out), nil
}

func (k *githubToolkit) getPullRequestDiff(ctx context.Context, args string) (string, error) {
	var p struct {
		PullNumber int `json:"pull_number"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("github_get_pr_diff: %w", err)
	}
	diff, _, err := k.client.PullRequests.GetRaw(ctx, k.owner, k.repo, p.PullNumber, githubv3.RawOptions{Type: githubv3.Diff})
	if err != nil {
		return "", fmt.Errorf("github_get_pr_diff: %w", err)
	}
	if diff == "" {
		return "no diff available", nil
	}
	return diff, nil
}

func (k *githubToolkit) postIssueComment(ctx context.Context, args string) (string, error) {
	var p struct {
		IssueNumber int    `json:"issue_number"`
		Body        string `json:"body"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("github_post_issue_comment: %w", err)
	}
	if strings.TrimSpace(p.Body) == "" {
		return "", fmt.Errorf("github_post_issue_comment: body must not be empty")
	}
	comment, _, err := k.client.Issues.CreateComment(ctx, k.owner, k.repo, p.IssueNumber, &githubv3.IssueComment{Body: githubv3.String(p.Body)})
	if err != nil {
		return "", fmt.Errorf("github_post_issue_comment: %w", err)
	}
	return fmt.Sprintf("comment posted: %s", comment.GetHTMLURL()), nil
}

func (k *githubToolkit) updateIssue(ctx context.Context, args string) (string, error) {
	var p struct {
		IssueNumber int    `json:"issue_number"`
		Title       string `json:"title"`
		Body        string `json:"body"`
		State       string `json:"state"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("github_update_issue: %w", err)
	}
	req := &githubv3.IssueRequest{}
	if p.Title != "" {
		req.Title = githubv3.String(p.Title)
	}
	if p.Body != "" {
		req.Body = githubv3.String(p.Body)
	}
	if p.State != "" {
		if p.State != "open" && p.State != "closed" {
			return "", fmt.Errorf("github_update_issue: state must be 'open' or 'closed', got %q", p.State)
		}
		req.State = githubv3.String(p.State)
	}
	if _, _, err := k.client.Issues.Edit(ctx, k.owner, k.repo, p.IssueNumber, req); err != nil {
		return "", fmt.Errorf("github_update_issue: %w", err)
	}
	return "issue updated", nil
}

func (k *githubToolkit) addLabels(ctx context.Context, args string) (string, error) {
	var p struct {
		IssueNumber int      `json:"issue_number"`
		Labels      []string `json:"labels"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("github_add_labels: %w", err)
	}
	if len(p.Labels) == 0 {
		return "", fmt.Errorf("github_add_labels: at least one label is required")
	}
	labels, _, err := k.client.Issues.AddLabelsToIssue(ctx, k.owner, k.repo, p.IssueNumber, p.Labels)
	if err != nil {
		return "", fmt.Errorf("github_add_labels: %w", err)
	}
	names := make([]string, 0, len(labels))
	for _, l := range labels {
		names = append(names, l.GetName())
	}
	return fmt.Sprintf("labels now on issue #%d: %s", p.IssueNumber, strings.Join(names, ", ")), nil
}

func (k *githubToolkit) removeLabel(ctx context.Context, args string) (string, error) {
	var p struct {
		IssueNumber int    `json:"issue_number"`
		Label       string `json:"label"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("github_remove_label: %w", err)
	}
	if _, err := k.client.Issues.RemoveLabelForIssue(ctx, k.owner, k.repo, p.IssueNumber, p.Label); err != nil {
		return "", fmt.Errorf("github_remove_label: %w", err)
	}
	return fmt.Sprintf("label %q removed from issue #%d", p.Label, p.IssueNumber), nil
}

func (k *githubToolkit) createPullRequest(ctx context.Context, args string) (string, error) {
	var p struct {
		Title string `json:"title"`
		Head  string `json:"head"`
		Base  string `json:"base"`
		Body  string `json:"body"`
		Draft bool   `json:"draft"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("github_create_pull_request: %w", err)
	}
	if p.Title == "" || p.Head == "" || p.Base == "" {
		return "", fmt.Errorf("github_create_pull_request: title, head and base are required")
	}
	pr, _, err := k.client.PullRequests.Create(ctx, k.owner, k.repo, &githubv3.NewPullRequest{
		Title: githubv3.String(p.Title),
		Head:  githubv3.String(p.Head),
		Base:  githubv3.String(p.Base),
		Body:  githubv3.String(p.Body),
		Draft: githubv3.Bool(p.Draft),
	})
	if err != nil {
		return "", fmt.Errorf("github_create_pull_request: %w", err)
	}
	return fmt.Sprintf("pull request #%d created: %s", pr.GetNumber(), pr.GetHTMLURL()), nil
}

func (k *githubToolkit) createPrReviewComment(ctx context.Context, args string) (string, error) {
	var p struct {
		PullNumber int    `json:"pull_number"`
		Body       string `json:"body"`
		Path       string `json:"path"`
		Line       int    `json:"line"`
		CommitID   string `json:"commit_id"`
		Side       string `json:"side"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("github_create_pr_review_comment: %w", err)
	}
	if strings.TrimSpace(p.Body) == "" || p.Path == "" {
		return "", fmt.Errorf("github_create_pr_review_comment: body and path are required")
	}
	comment := &githubv3.PullRequestComment{
		Body: githubv3.String(p.Body),
		Path: githubv3.String(p.Path),
		Line: githubv3.Int(p.Line),
	}
	if p.CommitID != "" {
		comment.CommitID = githubv3.String(p.CommitID)
	}
	if p.Side != "" {
		comment.Side = githubv3.String(p.Side)
	}
	created, _, err := k.client.PullRequests.CreateComment(ctx, k.owner, k.repo, p.PullNumber, comment)
	if err != nil {
		return "", fmt.Errorf("github_create_pr_review_comment: %w", err)
	}
	return fmt.Sprintf("review comment posted on %s: %s", created.GetPath(), created.GetHTMLURL()), nil
}

func (k *githubToolkit) submitReview(ctx context.Context, args string) (string, error) {
	var p struct {
		PullNumber int            `json:"pull_number"`
		Body       string         `json:"body"`
		Event      string         `json:"event"`
		Comments   []draftComment `json:"comments"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("github_submit_review: %w", err)
	}
	switch p.Event {
	case "APPROVE", "REQUEST_CHANGES", "COMMENT":
	default:
		return "", fmt.Errorf("github_submit_review: event must be 'APPROVE', 'REQUEST_CHANGES' or 'COMMENT', got %q", p.Event)
	}
	req := &githubv3.PullRequestReviewRequest{
		Body:  githubv3.String(p.Body),
		Event: githubv3.String(p.Event),
	}
	for _, c := range p.Comments {
		if c.Path == "" || c.Body == "" {
			continue
		}
		req.Comments = append(req.Comments, &githubv3.DraftReviewComment{
			Path: githubv3.String(c.Path),
			Line: githubv3.Int(c.Line),
			Body: githubv3.String(c.Body),
		})
	}
	review, _, err := k.client.PullRequests.CreateReview(ctx, k.owner, k.repo, p.PullNumber, req)
	if err != nil {
		return "", fmt.Errorf("github_submit_review: %w", err)
	}
	return fmt.Sprintf("review %q submitted: %s", review.GetState(), review.GetHTMLURL()), nil
}

type draftComment struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Body string `json:"body"`
}

func (k *githubToolkit) listPrReviews(ctx context.Context, args string) (string, error) {
	var p struct {
		PullNumber int `json:"pull_number"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("github_list_pr_reviews: %w", err)
	}
	reviews, _, err := k.client.PullRequests.ListReviews(ctx, k.owner, k.repo, p.PullNumber, nil)
	if err != nil {
		return "", fmt.Errorf("github_list_pr_reviews: %w", err)
	}
	type reviewView struct {
		Author    string `json:"author"`
		State     string `json:"state"`
		Body      string `json:"body"`
		Submitted string `json:"submitted_at,omitempty"`
	}
	views := make([]reviewView, 0, len(reviews))
	for _, r := range reviews {
		views = append(views, reviewView{Author: authorOf(r.GetUser()), State: r.GetState(), Body: r.GetBody(), Submitted: r.GetSubmittedAt().String()})
	}
	return k.json(views), nil
}

func (k *githubToolkit) getActorPermission(ctx context.Context, args string) (string, error) {
	var p struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("github_get_actor_permission: %w", err)
	}
	if p.Username == "" {
		return "", fmt.Errorf("github_get_actor_permission: username is required")
	}
	level, _, err := k.client.Repositories.GetPermissionLevel(ctx, k.owner, k.repo, p.Username)
	if err != nil {
		return "", fmt.Errorf("github_get_actor_permission: %w", err)
	}
	return fmt.Sprintf("%s has %q permission on %s/%s", p.Username, level.GetPermission(), k.owner, k.repo), nil
}

func (k *githubToolkit) getRelease(ctx context.Context, args string) (string, error) {
	var p struct {
		ReleaseID int64 `json:"release_id"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("github_get_release: %w", err)
	}
	release, _, err := k.client.Repositories.GetRelease(ctx, k.owner, k.repo, p.ReleaseID)
	if err != nil {
		return "", fmt.Errorf("github_get_release: %w", err)
	}
	return k.json(releaseView(release)), nil
}

func (k *githubToolkit) getReleaseByTag(ctx context.Context, args string) (string, error) {
	var p struct {
		Tag string `json:"tag"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("github_get_release_by_tag: %w", err)
	}
	if p.Tag == "" {
		return "", fmt.Errorf("github_get_release_by_tag: tag is required")
	}
	release, _, err := k.client.Repositories.GetReleaseByTag(ctx, k.owner, k.repo, p.Tag)
	if err != nil {
		return "", fmt.Errorf("github_get_release_by_tag: %w", err)
	}
	return k.json(releaseView(release)), nil
}

func (k *githubToolkit) updateRelease(ctx context.Context, args string) (string, error) {
	var p struct {
		ReleaseID int64  `json:"release_id"`
		Body      string `json:"body"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("github_update_release: %w", err)
	}
	if strings.TrimSpace(p.Body) == "" {
		return "", fmt.Errorf("github_update_release: body must not be empty")
	}
	release, _, err := k.client.Repositories.EditRelease(ctx, k.owner, k.repo, p.ReleaseID, &githubv3.RepositoryRelease{Body: githubv3.String(p.Body)})
	if err != nil {
		return "", fmt.Errorf("github_update_release: %w", err)
	}
	return fmt.Sprintf("release %q body updated: %s", release.GetTagName(), release.GetHTMLURL()), nil
}

func (k *githubToolkit) listReleases(ctx context.Context, args string) (string, error) {
	var p struct {
		PerPage int `json:"per_page"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("github_list_releases: %w", err)
	}
	if p.PerPage <= 0 {
		p.PerPage = 10
	}
	releases, _, err := k.client.Repositories.ListReleases(ctx, k.owner, k.repo, &githubv3.ListOptions{PerPage: p.PerPage})
	if err != nil {
		return "", fmt.Errorf("github_list_releases: %w", err)
	}
	views := make([]map[string]any, 0, len(releases))
	for _, r := range releases {
		v := releaseView(r)
		v["body"] = ""
		views = append(views, v)
	}
	return k.json(views), nil
}

func releaseView(r *githubv3.RepositoryRelease) map[string]any {
	return map[string]any{
		"id":         r.GetID(),
		"tag_name":   r.GetTagName(),
		"name":       r.GetName(),
		"body":       r.GetBody(),
		"draft":      r.GetDraft(),
		"prerelease": r.GetPrerelease(),
		"published":  r.GetPublishedAt().String(),
		"html_url":   r.GetHTMLURL(),
	}
}

func (k *githubToolkit) compareTags(ctx context.Context, args string) (string, error) {
	var p struct {
		Base string `json:"base"`
		Head string `json:"head"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("github_compare_tags: %w", err)
	}
	if p.Base == "" || p.Head == "" {
		return "", fmt.Errorf("github_compare_tags: base and head are required")
	}
	cmp, _, err := k.client.Repositories.CompareCommits(ctx, k.owner, k.repo, p.Base, p.Head, nil)
	if err != nil {
		return "", fmt.Errorf("github_compare_tags: %w", err)
	}
	type commitView struct {
		SHA     string `json:"sha"`
		Message string `json:"message"`
		Author  string `json:"author"`
		Date    string `json:"date"`
	}
	views := make([]commitView, 0, len(cmp.Commits))
	for _, c := range cmp.Commits {
		msg := ""
		if c.Commit != nil {
			msg = c.Commit.GetMessage()
			if i := strings.IndexByte(msg, '\n'); i >= 0 {
				msg = msg[:i]
			}
		}
		views = append(views, commitView{
			SHA:     c.GetSHA(),
			Message: msg,
			Author:  authorOf(c.Author),
			Date:    c.Commit.GetCommitter().GetDate().String(),
		})
	}
	return k.json(map[string]any{
		"status":        cmp.GetStatus(),
		"ahead_by":      cmp.GetAheadBy(),
		"behind_by":     cmp.GetBehindBy(),
		"total_commits": cmp.GetTotalCommits(),
		"commits":       views,
	}), nil
}

func authorOf(u *githubv3.User) string {
	if u == nil {
		return ""
	}
	return u.GetLogin()
}

func authorAssociation(assoc string) string {
	if assoc == "" {
		return "NONE"
	}
	return assoc
}

func imageURLs(body string, seen map[string]bool) []string {
	var out []string
	if body == "" {
		return out
	}
	for _, m := range markdownImageRE.FindAllStringSubmatch(body, -1) {
		if len(m) < 2 {
			continue
		}
		url := m[1]
		if seen[url] {
			continue
		}
		seen[url] = true
		out = append(out, url)
	}
	return out
}
