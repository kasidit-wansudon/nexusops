package git

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

var (
	ErrInvalidSignature   = errors.New("webhook: invalid signature")
	ErrMissingSignature   = errors.New("webhook: missing signature header")
	ErrInvalidToken       = errors.New("webhook: invalid token")
	ErrMissingToken       = errors.New("webhook: missing token header")
	ErrUnsupportedEvent   = errors.New("webhook: unsupported event type")
	ErrEmptyBody          = errors.New("webhook: empty request body")
	ErrInvalidJSON        = errors.New("webhook: invalid JSON payload")
	ErrGitCommandFailed   = errors.New("webhook: git command failed")
	ErrInvalidRepoPath    = errors.New("webhook: invalid repository path")
	ErrInvalidCommitHash  = errors.New("webhook: invalid commit hash")
)

// WebhookEvent represents a parsed webhook event from a Git provider.
type WebhookEvent struct {
	Type          string    `json:"type"`
	Repository    string    `json:"repository"`
	Branch        string    `json:"branch"`
	CommitHash    string    `json:"commit_hash"`
	CommitMessage string    `json:"commit_message"`
	Author        string    `json:"author"`
	PullRequest   *PRInfo   `json:"pull_request,omitempty"`
	Timestamp     time.Time `json:"timestamp"`
}

// PRInfo contains pull/merge request details.
type PRInfo struct {
	Number       int    `json:"number"`
	Title        string `json:"title"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
	Action       string `json:"action"`
}

// githubPushPayload represents the relevant fields from a GitHub push event.
type githubPushPayload struct {
	Ref        string `json:"ref"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	HeadCommit struct {
		ID        string `json:"id"`
		Message   string `json:"message"`
		Author    struct {
			Name string `json:"name"`
		} `json:"author"`
		Timestamp string `json:"timestamp"`
	} `json:"head_commit"`
}

// githubPRPayload represents the relevant fields from a GitHub pull_request event.
type githubPRPayload struct {
	Action      string `json:"action"`
	Repository  struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	PullRequest struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Head   struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
		CreatedAt string `json:"created_at"`
	} `json:"pull_request"`
}

// gitlabPushPayload represents the relevant fields from a GitLab push event.
type gitlabPushPayload struct {
	Ref        string `json:"ref"`
	Project    struct {
		PathWithNamespace string `json:"path_with_namespace"`
	} `json:"project"`
	Commits []struct {
		ID        string `json:"id"`
		Message   string `json:"message"`
		Author    struct {
			Name string `json:"name"`
		} `json:"author"`
		Timestamp string `json:"timestamp"`
	} `json:"commits"`
	UserName string `json:"user_name"`
}

// gitlabMRPayload represents the relevant fields from a GitLab merge_request event.
type gitlabMRPayload struct {
	Project struct {
		PathWithNamespace string `json:"path_with_namespace"`
	} `json:"project"`
	ObjectAttributes struct {
		IID          int    `json:"iid"`
		Title        string `json:"title"`
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
		LastCommit   struct {
			ID string `json:"id"`
		} `json:"last_commit"`
		Action    string `json:"action"`
		CreatedAt string `json:"created_at"`
	} `json:"object_attributes"`
	User struct {
		Name string `json:"name"`
	} `json:"user"`
}

// ParseGitHubWebhook validates the HMAC-SHA256 signature from the X-Hub-Signature-256
// header and parses the JSON body for push and pull_request events.
func ParseGitHubWebhook(r *http.Request, secret string) (*WebhookEvent, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("webhook: failed to read body: %w", err)
	}
	defer r.Body.Close()

	if len(body) == 0 {
		return nil, ErrEmptyBody
	}

	// Validate HMAC-SHA256 signature
	signatureHeader := r.Header.Get("X-Hub-Signature-256")
	if signatureHeader == "" {
		return nil, ErrMissingSignature
	}

	if !strings.HasPrefix(signatureHeader, "sha256=") {
		return nil, ErrInvalidSignature
	}
	signatureHex := strings.TrimPrefix(signatureHeader, "sha256=")

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signatureHex), []byte(expectedMAC)) {
		return nil, ErrInvalidSignature
	}

	// Determine event type
	eventType := r.Header.Get("X-GitHub-Event")

	switch eventType {
	case "push":
		return parseGitHubPush(body)
	case "pull_request":
		return parseGitHubPR(body)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedEvent, eventType)
	}
}

func parseGitHubPush(body []byte) (*WebhookEvent, error) {
	var payload githubPushPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}

	branch := strings.TrimPrefix(payload.Ref, "refs/heads/")

	ts, err := time.Parse(time.RFC3339, payload.HeadCommit.Timestamp)
	if err != nil {
		ts = time.Now().UTC()
	}

	return &WebhookEvent{
		Type:          "push",
		Repository:    payload.Repository.FullName,
		Branch:        branch,
		CommitHash:    payload.HeadCommit.ID,
		CommitMessage: payload.HeadCommit.Message,
		Author:        payload.HeadCommit.Author.Name,
		Timestamp:     ts,
	}, nil
}

func parseGitHubPR(body []byte) (*WebhookEvent, error) {
	var payload githubPRPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}

	ts, err := time.Parse(time.RFC3339, payload.PullRequest.CreatedAt)
	if err != nil {
		ts = time.Now().UTC()
	}

	return &WebhookEvent{
		Type:       "pull_request",
		Repository: payload.Repository.FullName,
		Branch:     payload.PullRequest.Head.Ref,
		CommitHash: payload.PullRequest.Head.SHA,
		Author:     payload.PullRequest.User.Login,
		PullRequest: &PRInfo{
			Number:       payload.PullRequest.Number,
			Title:        payload.PullRequest.Title,
			SourceBranch: payload.PullRequest.Head.Ref,
			TargetBranch: payload.PullRequest.Base.Ref,
			Action:       payload.Action,
		},
		Timestamp: ts,
	}, nil
}

// ParseGitLabWebhook validates the X-Gitlab-Token header and parses
// push and merge_request events from a GitLab webhook.
func ParseGitLabWebhook(r *http.Request, secret string) (*WebhookEvent, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("webhook: failed to read body: %w", err)
	}
	defer r.Body.Close()

	if len(body) == 0 {
		return nil, ErrEmptyBody
	}

	// Validate token
	token := r.Header.Get("X-Gitlab-Token")
	if token == "" {
		return nil, ErrMissingToken
	}

	if token != secret {
		return nil, ErrInvalidToken
	}

	// Determine event type
	eventType := r.Header.Get("X-Gitlab-Event")

	switch eventType {
	case "Push Hook":
		return parseGitLabPush(body)
	case "Merge Request Hook":
		return parseGitLabMR(body)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedEvent, eventType)
	}
}

func parseGitLabPush(body []byte) (*WebhookEvent, error) {
	var payload gitlabPushPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}

	branch := strings.TrimPrefix(payload.Ref, "refs/heads/")

	var commitHash, commitMessage, author string
	var ts time.Time

	if len(payload.Commits) > 0 {
		lastCommit := payload.Commits[len(payload.Commits)-1]
		commitHash = lastCommit.ID
		commitMessage = lastCommit.Message
		author = lastCommit.Author.Name
		var err error
		ts, err = time.Parse("2006-01-02T15:04:05Z07:00", lastCommit.Timestamp)
		if err != nil {
			ts = time.Now().UTC()
		}
	} else {
		author = payload.UserName
		ts = time.Now().UTC()
	}

	return &WebhookEvent{
		Type:          "push",
		Repository:    payload.Project.PathWithNamespace,
		Branch:        branch,
		CommitHash:    commitHash,
		CommitMessage: commitMessage,
		Author:        author,
		Timestamp:     ts,
	}, nil
}

func parseGitLabMR(body []byte) (*WebhookEvent, error) {
	var payload gitlabMRPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}

	ts, err := time.Parse("2006-01-02 15:04:05 MST", payload.ObjectAttributes.CreatedAt)
	if err != nil {
		ts = time.Now().UTC()
	}

	return &WebhookEvent{
		Type:       "merge_request",
		Repository: payload.Project.PathWithNamespace,
		Branch:     payload.ObjectAttributes.SourceBranch,
		CommitHash: payload.ObjectAttributes.LastCommit.ID,
		Author:     payload.User.Name,
		PullRequest: &PRInfo{
			Number:       payload.ObjectAttributes.IID,
			Title:        payload.ObjectAttributes.Title,
			SourceBranch: payload.ObjectAttributes.SourceBranch,
			TargetBranch: payload.ObjectAttributes.TargetBranch,
			Action:       payload.ObjectAttributes.Action,
		},
		Timestamp: ts,
	}, nil
}

// WebhookHandler implements http.Handler and dispatches parsed webhook events
// to a callback function. It supports both GitHub and GitLab webhooks,
// auto-detecting the provider based on request headers.
type WebhookHandler struct {
	secret   string
	callback func(*WebhookEvent)
}

// NewWebhookHandler creates a new WebhookHandler with the given secret and callback.
func NewWebhookHandler(secret string, callback func(*WebhookEvent)) *WebhookHandler {
	return &WebhookHandler{
		secret:   secret,
		callback: callback,
	}
}

// ServeHTTP implements the http.Handler interface. It auto-detects whether
// the request is from GitHub or GitLab and parses the webhook accordingly.
func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var event *WebhookEvent
	var err error

	// Auto-detect provider based on headers
	switch {
	case r.Header.Get("X-GitHub-Event") != "":
		event, err = ParseGitHubWebhook(r, h.secret)
	case r.Header.Get("X-Gitlab-Event") != "":
		event, err = ParseGitLabWebhook(r, h.secret)
	default:
		http.Error(w, "unsupported webhook provider", http.StatusBadRequest)
		return
	}

	if err != nil {
		if errors.Is(err, ErrInvalidSignature) || errors.Is(err, ErrInvalidToken) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if errors.Is(err, ErrMissingSignature) || errors.Is(err, ErrMissingToken) {
			http.Error(w, "missing authentication", http.StatusUnauthorized)
			return
		}
		if errors.Is(err, ErrUnsupportedEvent) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "failed to parse webhook", http.StatusBadRequest)
		return
	}

	if h.callback != nil {
		h.callback(event)
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"event":  event.Type,
	})
}

// GetCommitDiff returns the diff for a specific commit in the given repository.
// It shells out to git to retrieve the diff output.
func GetCommitDiff(repoPath, commitHash string) (string, error) {
	if repoPath == "" {
		return "", ErrInvalidRepoPath
	}
	if commitHash == "" {
		return "", ErrInvalidCommitHash
	}

	// Sanitize the commit hash: only allow hex characters
	for _, c := range commitHash {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return "", fmt.Errorf("%w: contains invalid characters", ErrInvalidCommitHash)
		}
	}

	cmd := exec.Command("git", "-C", repoPath, "diff", commitHash+"~1", commitHash)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If the commit is the initial commit, diff against empty tree
		if strings.Contains(string(output), "unknown revision") {
			emptyTree := "4b825dc642cb6eb9a060e54bf899d69f82cf7137"
			cmd = exec.Command("git", "-C", repoPath, "diff", emptyTree, commitHash)
			output, err = cmd.CombinedOutput()
			if err != nil {
				return "", fmt.Errorf("%w: %s", ErrGitCommandFailed, string(output))
			}
			return string(output), nil
		}
		return "", fmt.Errorf("%w: %s", ErrGitCommandFailed, string(output))
	}

	return string(output), nil
}
