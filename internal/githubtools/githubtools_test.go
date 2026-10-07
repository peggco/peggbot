package githubtools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/peggco/pegg/pkg/sdk"
)

func TestNewGitHubToolsRequiresToken(t *testing.T) {
	if _, err := NewGitHubTools(GitHubToolOptions{}); err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestNewGitHubToolsRequiresRepo(t *testing.T) {
	if _, err := NewGitHubTools(GitHubToolOptions{Token: "t"}); err == nil {
		t.Fatal("expected error for missing owner/repo")
	}
}

func TestNewGitHubToolsToolNames(t *testing.T) {
	tools, err := NewGitHubTools(GitHubToolOptions{Token: "t", Owner: "o", Repo: "r"})
	if err != nil {
		t.Fatalf("NewGitHubTools: %v", err)
	}
	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Name()] = true
	}
	for _, want := range []string{
		"github_get_issue",
		"github_get_pull_request",
		"github_get_pr_diff",
		"github_post_issue_comment",
		"github_update_issue",
		"github_add_labels",
		"github_remove_label",
		"github_create_pull_request",
		"github_create_pr_review_comment",
		"github_submit_review",
		"github_list_pr_reviews",
		"github_get_actor_permission",
		"github_get_release",
		"github_get_release_by_tag",
		"github_update_release",
		"github_list_releases",
		"github_compare_tags",
	} {
		if !names[want] {
			t.Errorf("missing tool %q", want)
		}
	}
}

func TestGitHubGetIssue(t *testing.T) {
	var gotPaths []string
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.URL.Path)
		switch r.URL.Path {
		case "/repos/o/r/issues/42":
			w.Write([]byte(`{"number":42,"title":"Broken build","body":"See ![](https://user-images.githubusercontent.com/1/a.png)","state":"open","user":{"login":"alice"},"labels":[{"name":"bug"}]}`))
		case "/repos/o/r/issues/42/comments":
			w.Write([]byte(`[{"body":"I can reproduce","user":{"login":"bob"}}]`))
		default:
			http.NotFound(w, r)
		}
	})
	defer server.Close()

	tools := newTestGitHubTools(t, server.URL)
	issue, err := findTool(tools, "github_get_issue")
	if err != nil {
		t.Fatal(err)
	}
	out, err := issue.Execute(context.Background(), `{"issue_number":42}`)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(gotPaths) != 2 || gotPaths[0] != "/repos/o/r/issues/42" || gotPaths[1] != "/repos/o/r/issues/42/comments" {
		t.Errorf("unexpected paths: %v", gotPaths)
	}
	var view struct {
		Title       string   `json:"title"`
		Attachments []string `json:"attachments"`
		Comments    []struct {
			Author string `json:"author"`
		} `json:"comments"`
	}
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if view.Title != "Broken build" {
		t.Errorf("title = %q", view.Title)
	}
	if len(view.Attachments) != 1 || !strings.Contains(view.Attachments[0], "a.png") {
		t.Errorf("attachments = %v", view.Attachments)
	}
	if len(view.Comments) != 1 || view.Comments[0].Author != "bob" {
		t.Errorf("comments = %v", view.Comments)
	}
}

func TestGitHubPostIssueComment(t *testing.T) {
	var gotBody string
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "expected POST", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/repos/o/r/issues/7/comments" {
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		var req struct {
			Body string `json:"body"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotBody = req.Body
		w.Write([]byte(`{"html_url":"https://github.com/o/r/issues/7#issuecomment-1"}`))
	})
	defer server.Close()

	tools := newTestGitHubTools(t, server.URL)
	tool, err := findTool(tools, "github_post_issue_comment")
	if err != nil {
		t.Fatal(err)
	}
	out, err := tool.Execute(context.Background(), `{"issue_number":7,"body":"**fixed** in #8"}`)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotBody != "**fixed** in #8" {
		t.Errorf("body = %q", gotBody)
	}
	if !strings.Contains(out, "issuecomment-1") {
		t.Errorf("unexpected output: %s", out)
	}
}

func TestGitHubCreatePullRequest(t *testing.T) {
	var got struct {
		Title string `json:"title"`
		Head  string `json:"head"`
		Base  string `json:"base"`
		Body  string `json:"body"`
	}
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/repos/o/r/pulls" {
			http.Error(w, "unexpected request", http.StatusNotFound)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(`{"number":9,"html_url":"https://github.com/o/r/pull/9"}`))
	})
	defer server.Close()

	tools := newTestGitHubTools(t, server.URL)
	tool, err := findTool(tools, "github_create_pull_request")
	if err != nil {
		t.Fatal(err)
	}
	out, err := tool.Execute(context.Background(), `{"title":"Fix build","head":"peggbot/issue-42","base":"main","body":"Closes #42"}`)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got.Title != "Fix build" || got.Head != "peggbot/issue-42" || got.Base != "main" || got.Body != "Closes #42" {
		t.Errorf("request = %+v", got)
	}
	if !strings.Contains(out, "/pull/9") {
		t.Errorf("unexpected output: %s", out)
	}
}

func TestGitHubGetActorPermission(t *testing.T) {
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/o/r/collaborators/octocat/permission" {
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		w.Write([]byte(`{"permission":"admin","user":{"login":"octocat"}}`))
	})
	defer server.Close()

	tools := newTestGitHubTools(t, server.URL)
	tool, err := findTool(tools, "github_get_actor_permission")
	if err != nil {
		t.Fatal(err)
	}
	out, err := tool.Execute(context.Background(), `{"username":"octocat"}`)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out, `"admin"`) {
		t.Errorf("unexpected output: %s", out)
	}
}

func TestGitHubCompareTags(t *testing.T) {
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/o/r/compare/v0.1.2...v0.1.3" {
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Write([]byte(`{"ahead_by":3,"total_commits":3,"commits":[{"sha":"aaa","commit":{"message":"feat: add thing","committer":{"date":"2026-01-01T00:00:00Z"}}},{"sha":"bbb","commit":{"message":"fix: repair thing"}}]}`))
	})
	defer server.Close()

	tools := newTestGitHubTools(t, server.URL)
	tool, err := findTool(tools, "github_compare_tags")
	if err != nil {
		t.Fatal(err)
	}
	out, err := tool.Execute(context.Background(), `{"base":"v0.1.2","head":"v0.1.3"}`)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out, "feat: add thing") {
		t.Errorf("unexpected output: %s", out)
	}
	if strings.Contains(out, "\nfeat:") {
		t.Errorf("subject truncation failed: %s", out)
	}
}

func TestGitHubUpdateRelease(t *testing.T) {
	var gotBody string
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/repos/o/r/releases/5" {
			http.Error(w, "unexpected request", http.StatusNotFound)
			return
		}
		var req struct {
			Body string `json:"body"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotBody = req.Body
		w.Write([]byte(`{"id":5,"tag_name":"v0.1.3","html_url":"https://github.com/o/r/releases/tag/v0.1.3"}`))
	})
	defer server.Close()

	tools := newTestGitHubTools(t, server.URL)
	tool, err := findTool(tools, "github_update_release")
	if err != nil {
		t.Fatal(err)
	}
	out, err := tool.Execute(context.Background(), `{"release_id":5,"body":"## New Features"}`)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotBody != "## New Features" {
		t.Errorf("body = %q", gotBody)
	}
	if !strings.Contains(out, "v0.1.3") {
		t.Errorf("unexpected output: %s", out)
	}
}

func TestImageURLs(t *testing.T) {
	body := "see ![screenshot](https://user-images.githubusercontent.com/1/a.png) and ![alt](https://example.com/b.jpg) and https://not-an-image.com/x"
	seen := map[string]bool{}
	urls := imageURLs(body, seen)
	if len(urls) != 2 {
		t.Fatalf("urls = %v", urls)
	}
	dupes := imageURLs(body, seen)
	if len(dupes) != 0 {
		t.Fatalf("dedupe broken: %v", dupes)
	}
}

func newTestGitHubTools(t *testing.T, serverURL string) []sdk.Tool {
	t.Helper()
	tools, err := NewGitHubTools(GitHubToolOptions{
		Token:   "test-token",
		Owner:   "o",
		Repo:    "r",
		BaseURL: serverURL + "/api/v3/",
	})
	if err != nil {
		t.Fatalf("NewGitHubTools: %v", err)
	}
	return tools
}

func newTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = strings.TrimPrefix(r.URL.Path, "/api/v3")
		handler(w, r)
	}))
}

func findTool(tools []sdk.Tool, name string) (sdk.Tool, error) {
	for _, tool := range tools {
		if tool.Name() == name {
			return tool, nil
		}
	}
	return nil, &findToolError{name: name}
}

type findToolError struct{ name string }

func (e *findToolError) Error() string { return "tool not found: " + e.name }
