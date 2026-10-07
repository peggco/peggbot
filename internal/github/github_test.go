package github

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEvent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "event.json")
	payload := `{
		"action": "created",
		"issue": {"number": 7, "title": "Bug", "body": "It breaks"},
		"comment": {"id": 99, "body": "@peggbot is this really an issue?"},
		"sender": {"login": "admin"},
		"label": null
	}`
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	ev, err := LoadEvent(path)
	if err != nil {
		t.Fatalf("LoadEvent: %v", err)
	}
	if ev.IssueNumber() != 7 {
		t.Errorf("issue number = %d", ev.IssueNumber())
	}
	if ev.Actor() != "admin" {
		t.Errorf("actor = %q", ev.Actor())
	}
	if ev.IsPullRequestConversation() {
		t.Error("expected issue, got pull request")
	}
	if !Mentions(ev.Comment.Body, "peggbot") {
		t.Error("expected mention detection")
	}
	if ev.Raw == nil {
		t.Error("raw payload not captured")
	}
}

func TestLoadEventPullRequestConversation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "event.json")
	payload := `{"action":"created","issue":{"number":3,"title":"PR","body":"","pull_request":{"url":"https://api.github.com/repos/o/r/pulls/3"}},"comment":{"body":"hi @peggbot"},"sender":{"login":"bob"}}`
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	ev, err := LoadEvent(path)
	if err != nil {
		t.Fatalf("LoadEvent: %v", err)
	}
	if !ev.IsPullRequestConversation() {
		t.Error("expected pull request conversation")
	}
}

func TestMentions(t *testing.T) {
	cases := []struct {
		body string
		want bool
	}{
		{"@peggbot please check", true},
		{"ping @peggbot now", true},
		{"@peggbot", true},
		{"@peggbot!", true},
		{"@peggbot, thanks", true},
		{"hello peggbot", false},
		{"@peggbotter", false},
		{"xpeggbot", false},
		{"", false},
	}
	for _, c := range cases {
		if got := Mentions(c.body, "peggbot"); got != c.want {
			t.Errorf("Mentions(%q) = %v, want %v", c.body, got, c.want)
		}
	}
}

func TestHasPermission(t *testing.T) {
	if !HasPermission("admin", "admin") {
		t.Error("admin should satisfy admin")
	}
	if HasPermission("read", "admin") {
		t.Error("read must not satisfy admin")
	}
	if !HasPermission("admin", "write") {
		t.Error("admin should satisfy write")
	}
	if !HasPermission("write", "read") {
		t.Error("write should satisfy read")
	}
	if HasPermission("none", "read") {
		t.Error("none must not satisfy read")
	}
}
