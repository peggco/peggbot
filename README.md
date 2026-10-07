# PeggBot

A GitHub bot built on the [Pegg Go SDK](https://github.com/peggco/pegg).

## What it does

| Mode | Trigger | Behavior |
|---|---|---|
| `solve-issue` | Issue labeled `peggbot` | Solves the issue end-to-end, opens a PR (`Closes #n`), comments with the PR link |
| `review-pr` | `peggbot` added as PR reviewer | Runs the `/review` skill: inline comments + `APPROVE` / `REQUEST_CHANGES` / `COMMENT` verdict |
| `respond` | `@peggbot` mention in an issue/PR comment | Answers with the full thread + image attachments as context — **only for repo admins/owners** (configurable) |
| `release-notes` | Release published | Rewrites the auto-generated (commit-message-only) release body into structured notes |

The agent gets the full GitHub toolkit as tools: `github_post_issue_comment`,
`github_create_pull_request`, `github_create_pr_review_comment`,
`github_submit_review`, `github_get_issue`, `github_get_pr_diff`,
`github_update_release`, `github_compare_tags`, and more. The toolkit lives in
this repository at [`internal/githubtools/`](internal/githubtools/) and is
registered into the agent at startup via `Engine.RegisterTool` — the agent
drives it itself, so every mode is fully autonomous.

## Usage

Add the action to a workflow:

```yaml
name: PeggBot Solve Issue

on:
  issues:
    types: [labeled]

permissions:
  contents: write
  pull-requests: write
  issues: write

jobs:
  solve:
    if: github.event.label.name == 'peggbot'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: peggco/peggbot@v1
        with:
          mode: solve-issue
          token: ${{ secrets.GITHUB_TOKEN }}
          llm-provider: anthropic
          llm-model: ${{ vars.PEGGBOT_MODEL }}
          llm-api-key: ${{ secrets.PEGGBOT_API_KEY }}
          prompt-file: .github/prompts/solve-issue.md
```

Ready-made workflows for all four modes live in [`.github/workflows/`](.github/workflows/).

### Repository setup

1. Create a `peggbot` label (`gh label create peggbot`).
2. Add the bot account as a repository collaborator so it can be requested as
   a reviewer (Read is enough for reviews; use Write if you also want it to
   self-assign).
3. Store the LLM credentials as a secret (`PEGGBOT_API_KEY`) and the model as
   a variable (`PEGGBOT_MODEL`).
4. Point `bot-login` at the bot account (default `peggbot`).

### Prompt files (`.github/prompts/`)

Every mode accepts `prompt-file`: a Markdown file inside the repo whose
content is appended to the mode's built-in system prompt. Write repository
rules there, and reference different files from different workflows.

`skills-dir` additionally loads `SKILL.md` skill folders (one per
subdirectory) into the agent via `sdk.LoadSkills`:

```
.github/prompts/
├── solve-issue.md          # prompt-file for the solve workflow
├── review-pr.md
├── respond.md
├── release-notes.md
└── release-notes/          # optional SKILL.md skill folder
    └── SKILL.md
```

### Permissions

`respond` mode only answers users whose permission level satisfies
`required-permission` (default `admin`; repository owners and org admins count
as admin). On denial it either skips silently or posts a polite decline
(`unauthorized-action: comment`). The bot never answers its own comments.

## Inputs

| Input | Default | Description |
|---|---|---|
| `mode` | — | `solve-issue`, `review-pr`, `respond`, `release-notes` |
| `token` | — | `GITHUB_TOKEN` or a bot PAT |
| `llm-provider` | — | e.g. `anthropic`, `openai` |
| `llm-model` | — | e.g. `claude-sonnet-4-5` |
| `llm-api-key` | — | provider key (use a secret) |
| `llm-base-url` | — | optional provider/proxy base URL |
| `prompt-file` | — | extra prompt, relative to the workspace |
| `skills-dir` | — | SKILL.md skill directory |
| `issue-number` / `pull-request-number` / `release-id` / `comment-id` | from event | trigger context override |
| `required-permission` | `admin` | minimum actor permission for `respond` |
| `unauthorized-action` | `skip` | `skip` or `comment` |
| `bot-login` | `peggbot` | bot identity (loop guard) |
| `branch-prefix` | `peggbot` | branches become `<prefix>/issue-<n>` |
| `base-branch` | `main` | PR base for `solve-issue` |
| `max-iterations` | `80` | agent loop cap |
| `timeout-minutes` | `30` | agent runtime cap |
| `max-attachments` | `5` | images pulled from a thread |
| `dry-run` | `false` | no writes to GitHub |
| `github-server-url` | `https://github.com` | git push auth target (GHES) |
| `ghes-api-base-url` | — | GitHub Enterprise API root |

## Development

The module imports the Pegg SDK from pkg.go.dev:

```go
import "github.com/peggco/pegg/pkg/sdk"
```

The SDK is used as published (`v0.1.5`); no unreleased SDK surface is
required. The GitHub toolkit is a plain Go package in this repository built on
`sdk.NewTool` and `go-github`.

```bash
go build ./... && go vet ./... && go test ./...
```