package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Mode        string
	Token       string
	Owner       string
	Repo        string
	Actor       string
	Workspace   string
	EventPath   string
	ActionPath  string
	GitServer   string
	GHESBaseURL string
	DryRun      bool

	Provider string
	Model    string
	APIKey   string
	BaseURL  string

	PromptFile string
	SkillsDir  string

	IssueNumber        int
	PullRequestNumber  int
	ReleaseID          int64
	CommentID          int64
	RequiredPermission string
	UnauthorizedAction string
	BotLogin           string
	BranchPrefix       string
	BaseBranch         string
	MaxIterations      int
	Timeout            time.Duration
	MaxAttachments     int
}

func Load() (*Config, error) {
	repo := getenv("GITHUB_REPOSITORY")
	owner, name, err := splitRepo(repo)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Mode:               getenv("INPUT_MODE"),
		Token:              getenv("INPUT_TOKEN"),
		Owner:              owner,
		Repo:               name,
		Actor:              getenv("GITHUB_ACTOR"),
		Workspace:          firstNonEmpty(getenv("INPUT_WORKSPACE"), getenv("GITHUB_WORKSPACE"), "."),
		EventPath:          getenv("GITHUB_EVENT_PATH"),
		ActionPath:         getenv("GITHUB_ACTION_PATH"),
		GitServer:          firstNonEmpty(getenv("INPUT_GITHUB_SERVER_URL"), "https://github.com"),
		GHESBaseURL:        getenv("INPUT_GHES_API_BASE_URL"),
		DryRun:             boolEnv("INPUT_DRY_RUN", false),
		Provider:           getenv("INPUT_LLM_PROVIDER"),
		Model:              getenv("INPUT_LLM_MODEL"),
		APIKey:             getenv("INPUT_LLM_API_KEY"),
		BaseURL:            getenv("INPUT_LLM_BASE_URL"),
		PromptFile:         getenv("INPUT_PROMPT_FILE"),
		SkillsDir:          getenv("INPUT_SKILLS_DIR"),
		RequiredPermission: firstNonEmpty(getenv("INPUT_REQUIRED_PERMISSION"), "admin"),
		UnauthorizedAction: firstNonEmpty(getenv("INPUT_UNAUTHORIZED_ACTION"), "skip"),
		BotLogin:           firstNonEmpty(getenv("INPUT_BOT_LOGIN"), "peggbot"),
		BranchPrefix:       firstNonEmpty(getenv("INPUT_BRANCH_PREFIX"), "peggbot"),
		BaseBranch:         firstNonEmpty(getenv("INPUT_BASE_BRANCH"), "main"),
		MaxIterations:      intEnv("INPUT_MAX_ITERATIONS", 80),
		Timeout:            time.Duration(intEnv("INPUT_TIMEOUT_MINUTES", 30)) * time.Minute,
		MaxAttachments:     intEnv("INPUT_MAX_ATTACHMENTS", 5),
	}

	cfg.IssueNumber = intEnv("INPUT_ISSUE_NUMBER", 0)
	cfg.PullRequestNumber = intEnv("INPUT_PULL_REQUEST_NUMBER", 0)
	cfg.ReleaseID = int64Env("INPUT_RELEASE_ID", 0)
	cfg.CommentID = int64Env("INPUT_COMMENT_ID", 0)

	if cfg.Workspace != "" && !filepath.IsAbs(cfg.Workspace) {
		abs, err := filepath.Abs(cfg.Workspace)
		if err != nil {
			return nil, fmt.Errorf("config: resolve workspace: %w", err)
		}
		cfg.Workspace = abs
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	switch c.Mode {
	case "solve-issue", "review-pr", "respond", "release-notes":
	default:
		return fmt.Errorf("config: unknown mode %q (want solve-issue, review-pr, respond or release-notes)", c.Mode)
	}
	if c.Token == "" {
		return fmt.Errorf("config: token is required")
	}
	if c.Owner == "" || c.Repo == "" {
		return fmt.Errorf("config: GITHUB_REPOSITORY is required, got %q", getenv("GITHUB_REPOSITORY"))
	}
	if c.Provider == "" {
		return fmt.Errorf("config: llm-provider is required")
	}
	if c.Model == "" {
		return fmt.Errorf("config: llm-model is required")
	}
	if c.APIKey == "" {
		return fmt.Errorf("config: llm-api-key is required")
	}
	switch c.RequiredPermission {
	case "admin", "write", "read":
	default:
		return fmt.Errorf("config: required-permission must be admin, write or read, got %q", c.RequiredPermission)
	}
	switch c.UnauthorizedAction {
	case "skip", "comment":
	default:
		return fmt.Errorf("config: unauthorized-action must be skip or comment, got %q", c.UnauthorizedAction)
	}
	if c.MaxIterations <= 0 {
		return fmt.Errorf("config: max-iterations must be positive")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("config: timeout-minutes must be positive")
	}
	return nil
}

func (c *Config) PromptPath() string {
	if c.PromptFile == "" {
		return ""
	}
	if filepath.IsAbs(c.PromptFile) {
		return c.PromptFile
	}
	return filepath.Join(c.Workspace, c.PromptFile)
}

func (c *Config) SkillsPath() string {
	if c.SkillsDir == "" {
		return ""
	}
	if filepath.IsAbs(c.SkillsDir) {
		return c.SkillsDir
	}
	return filepath.Join(c.Workspace, c.SkillsDir)
}

func (c *Config) LoadPromptFile() (string, error) {
	path := c.PromptPath()
	if path == "" {
		return "", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("config: read prompt file %s: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func splitRepo(repo string) (string, string, error) {
	if repo == "" {
		return "", "", fmt.Errorf("config: GITHUB_REPOSITORY is not set")
	}
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("config: malformed GITHUB_REPOSITORY %q", repo)
	}
	return parts[0], parts[1], nil
}

func getenv(key string) string { return os.Getenv(key) }

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func boolEnv(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func intEnv(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func int64Env(key string, fallback int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fallback
	}
	return n
}
