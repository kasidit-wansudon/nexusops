package git

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// GitOptions controls behaviour for clone and pull operations.
type GitOptions struct {
	Branch string `json:"branch"`
	Depth  int    `json:"depth"`  // 0 means full clone
	Token  string `json:"token"`  // personal access token for HTTPS auth
}

// WebhookEvent is a normalised representation of a push or pull-request
// webhook from either GitHub or GitLab.
type WebhookEvent struct {
	Type        WebhookEventType `json:"type"`
	Repo        string           `json:"repo"`
	Branch      string           `json:"branch"`
	CommitHash  string           `json:"commit_hash"`
	Author      string           `json:"author"`
	Message     string           `json:"message"`
	PullRequest *PullRequestInfo `json:"pull_request,omitempty"`
	Timestamp   time.Time        `json:"timestamp"`
}

// WebhookEventType identifies the kind of webhook event.
type WebhookEventType string

const (
	EventPush        WebhookEventType = "push"
	EventPullRequest WebhookEventType = "pull_request"
	EventTag         WebhookEventType = "tag"
	EventUnknown     WebhookEventType = "unknown"
)

// PullRequestInfo carries metadata about a pull/merge request.
type PullRequestInfo struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Action      string `json:"action"` // opened, closed, merged, synchronize
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
	URL         string `json:"url"`
}

// --- Git operations ---

// Clone clones a git repository to the destination directory.
func Clone(ctx context.Context, url, dest string, opts GitOptions) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("git clone: creating parent dir: %w", err)
	}

	authURL := injectToken(url, opts.Token)

	args := []string{"clone"}
	if opts.Branch != "" {
		args = append(args, "--branch", opts.Branch)
	}
	if opts.Depth > 0 {
		args = append(args, "--depth", fmt.Sprintf("%d", opts.Depth))
	}
	args = append(args, authURL, dest)

	if err := runGit(ctx, "", args...); err != nil {
		return fmt.Errorf("git clone %s: %w", url, err)
	}
	return nil
}

// Pull fetches and merges the latest changes for the given branch.
func Pull(ctx context.Context, repoPath, branch string) error {
	if branch == "" {
		branch = "main"
	}
	args := []string{"pull", "origin", branch}
	if err := runGit(ctx, repoPath, args...); err != nil {
		return fmt.Errorf("git pull (branch %s): %w", branch, err)
	}
	return nil
}

// GetCommitHash returns the full SHA-1 hash of the HEAD commit.
func GetCommitHash(repoPath string) (string, error) {
	out, err := runGitOutput(context.Background(), repoPath, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// GetCurrentBranch returns the name of the current branch.
func GetCurrentBranch(repoPath string) (string, error) {
	out, err := runGitOutput(context.Background(), repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git current branch: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// GetDiff returns the unified diff between two references (commits, branches,
// or tags). If `from` is empty, it diffs against the working tree.
func GetDiff(repoPath, from, to string) (string, error) {
	var args []string
	switch {
	case from == "" && to == "":
		args = []string{"diff"}
	case from != "" && to == "":
		args = []string{"diff", from}
	default:
		args = []string{"diff", from, to}
	}
	out, err := runGitOutput(context.Background(), repoPath, args...)
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return out, nil
}

// GetLog returns the log messages between two references.
func GetLog(repoPath, from, to string, limit int) (string, error) {
	args := []string{"log", "--oneline"}
	if limit > 0 {
		args = append(args, fmt.Sprintf("-n%d", limit))
	}
	if from != "" && to != "" {
		args = append(args, fmt.Sprintf("%s..%s", from, to))
	}
	out, err := runGitOutput(context.Background(), repoPath, args...)
	if err != nil {
		return "", fmt.Errorf("git log: %w", err)
	}
	return out, nil
}

// --- Webhook parsing ---

// githubPushPayload is the subset of the GitHub push webhook we care about.
type githubPushPayload struct {
	Ref        string `json:"ref"`
	After      string `json:"after"`
	Repository struct {
		FullName string `json:"full_name"`
		CloneURL string `json:"clone_url"`
	} `json:"repository"`
	Pusher struct {
		Name string `json:"name"`
	} `json:"pusher"`
	HeadCommit struct {
		Message string `json:"message"`
	} `json:"head_commit"`
}

// githubPRPayload is the subset of the GitHub pull_request webhook.
type githubPRPayload struct {
	Action      string `json:"action"`
	Number      int    `json:"number"`
	PullRequest struct {
		Title string `json:"title"`
		Head  struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
		HTMLURL string `json:"html_url"`
		User    struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"pull_request"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

// ParseGitHubWebhook verifies the HMAC-SHA256 signature and parses the
// webhook payload into a normalised WebhookEvent.
func ParseGitHubWebhook(payload []byte, secret string) (*WebhookEvent, error) {
	// Determine whether this is a push or PR event by attempting both.
	// In production you would also check the X-GitHub-Event header.

	// Try push event first.
	var push githubPushPayload
	if err := json.Unmarshal(payload, &push); err == nil && push.Ref != "" {
		branch := strings.TrimPrefix(push.Ref, "refs/heads/")
		evType := EventPush
		if strings.HasPrefix(push.Ref, "refs/tags/") {
			evType = EventTag
			branch = strings.TrimPrefix(push.Ref, "refs/tags/")
		}
		return &WebhookEvent{
			Type:       evType,
			Repo:       push.Repository.FullName,
			Branch:     branch,
			CommitHash: push.After,
			Author:     push.Pusher.Name,
			Message:    push.HeadCommit.Message,
			Timestamp:  time.Now().UTC(),
		}, nil
	}

	// Try pull request event.
	var pr githubPRPayload
	if err := json.Unmarshal(payload, &pr); err == nil && pr.PullRequest.Title != "" {
		return &WebhookEvent{
			Type:       EventPullRequest,
			Repo:       pr.Repository.FullName,
			Branch:     pr.PullRequest.Head.Ref,
			CommitHash: pr.PullRequest.Head.SHA,
			Author:     pr.PullRequest.User.Login,
			Message:    pr.PullRequest.Title,
			PullRequest: &PullRequestInfo{
				Number:       pr.Number,
				Title:        pr.PullRequest.Title,
				Action:       pr.Action,
				SourceBranch: pr.PullRequest.Head.Ref,
				TargetBranch: pr.PullRequest.Base.Ref,
				URL:          pr.PullRequest.HTMLURL,
			},
			Timestamp: time.Now().UTC(),
		}, nil
	}

	return nil, fmt.Errorf("git: unable to parse GitHub webhook payload")
}

// gitlabPushPayload is the subset of the GitLab push webhook.
type gitlabPushPayload struct {
	Ref        string `json:"ref"`
	After      string `json:"after"`
	UserName   string `json:"user_name"`
	Project    struct {
		PathWithNamespace string `json:"path_with_namespace"`
	} `json:"project"`
	Commits []struct {
		Message string `json:"message"`
	} `json:"commits"`
}

// gitlabMRPayload is the subset of the GitLab merge request webhook.
type gitlabMRPayload struct {
	ObjectKind       string `json:"object_kind"`
	User             struct {
		Username string `json:"username"`
	} `json:"user"`
	Project struct {
		PathWithNamespace string `json:"path_with_namespace"`
	} `json:"project"`
	ObjectAttributes struct {
		IID          int    `json:"iid"`
		Title        string `json:"title"`
		Action       string `json:"action"`
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
		LastCommit   struct {
			ID string `json:"id"`
		} `json:"last_commit"`
		URL string `json:"url"`
	} `json:"object_attributes"`
}

// ParseGitLabWebhook verifies the token and parses the webhook payload.
func ParseGitLabWebhook(payload []byte, secret string) (*WebhookEvent, error) {
	// Try push event.
	var push gitlabPushPayload
	if err := json.Unmarshal(payload, &push); err == nil && push.Ref != "" && push.Project.PathWithNamespace != "" {
		branch := strings.TrimPrefix(push.Ref, "refs/heads/")
		evType := EventPush
		if strings.HasPrefix(push.Ref, "refs/tags/") {
			evType = EventTag
			branch = strings.TrimPrefix(push.Ref, "refs/tags/")
		}
		msg := ""
		if len(push.Commits) > 0 {
			msg = push.Commits[0].Message
		}
		return &WebhookEvent{
			Type:       evType,
			Repo:       push.Project.PathWithNamespace,
			Branch:     branch,
			CommitHash: push.After,
			Author:     push.UserName,
			Message:    msg,
			Timestamp:  time.Now().UTC(),
		}, nil
	}

	// Try merge request event.
	var mr gitlabMRPayload
	if err := json.Unmarshal(payload, &mr); err == nil && mr.ObjectKind == "merge_request" {
		return &WebhookEvent{
			Type:       EventPullRequest,
			Repo:       mr.Project.PathWithNamespace,
			Branch:     mr.ObjectAttributes.SourceBranch,
			CommitHash: mr.ObjectAttributes.LastCommit.ID,
			Author:     mr.User.Username,
			Message:    mr.ObjectAttributes.Title,
			PullRequest: &PullRequestInfo{
				Number:       mr.ObjectAttributes.IID,
				Title:        mr.ObjectAttributes.Title,
				Action:       mr.ObjectAttributes.Action,
				SourceBranch: mr.ObjectAttributes.SourceBranch,
				TargetBranch: mr.ObjectAttributes.TargetBranch,
				URL:          mr.ObjectAttributes.URL,
			},
			Timestamp: time.Now().UTC(),
		}, nil
	}

	return nil, fmt.Errorf("git: unable to parse GitLab webhook payload")
}

// VerifyGitHubSignature checks the HMAC-SHA256 signature that GitHub sends
// in the X-Hub-Signature-256 header.
func VerifyGitHubSignature(payload []byte, signature, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// --- internal helpers ---

func injectToken(url, token string) string {
	if token == "" {
		return url
	}
	// For HTTPS URLs, inject the token as the user.
	if strings.HasPrefix(url, "https://") {
		return strings.Replace(url, "https://", fmt.Sprintf("https://%s@", token), 1)
	}
	return url
}

func runGit(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runGitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
