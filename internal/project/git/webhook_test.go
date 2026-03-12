package git

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// signPayload computes the HMAC-SHA256 signature for a payload using the
// given secret, returning the "sha256=..." header value.
func signPayload(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// --- Tests ---

func TestNewWebhookHandler(t *testing.T) {
	var called bool
	h := NewWebhookHandler("my-secret", func(e *WebhookEvent) {
		called = true
	})
	if h == nil {
		t.Fatal("NewWebhookHandler returned nil")
	}
	if h.secret != "my-secret" {
		t.Errorf("secret = %q, want %q", h.secret, "my-secret")
	}
	if h.callback == nil {
		t.Error("callback should not be nil")
	}
	// Invoke the callback to verify it was wired correctly.
	h.callback(&WebhookEvent{})
	if !called {
		t.Error("callback was not invoked")
	}
}

func TestParseGitHubWebhook_Push(t *testing.T) {
	secret := "test-secret"
	payload := `{
		"ref": "refs/heads/main",
		"repository": {"full_name": "org/repo"},
		"head_commit": {
			"id": "abc123def456",
			"message": "fix: resolve issue #42",
			"author": {"name": "Alice"},
			"timestamp": "2025-03-10T12:00:00Z"
		}
	}`
	body := []byte(payload)
	sig := signPayload(secret, body)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sig)
	req.Header.Set("X-GitHub-Event", "push")

	event, err := ParseGitHubWebhook(req, secret)
	if err != nil {
		t.Fatalf("ParseGitHubWebhook returned error: %v", err)
	}
	if event.Type != "push" {
		t.Errorf("Type = %q, want %q", event.Type, "push")
	}
	if event.Repository != "org/repo" {
		t.Errorf("Repository = %q, want %q", event.Repository, "org/repo")
	}
	if event.Branch != "main" {
		t.Errorf("Branch = %q, want %q", event.Branch, "main")
	}
	if event.CommitHash != "abc123def456" {
		t.Errorf("CommitHash = %q, want %q", event.CommitHash, "abc123def456")
	}
	if event.Author != "Alice" {
		t.Errorf("Author = %q, want %q", event.Author, "Alice")
	}
	if event.PullRequest != nil {
		t.Error("PullRequest should be nil for push event")
	}
}

func TestParseGitHubWebhook_PullRequest(t *testing.T) {
	secret := "pr-secret"
	payload := `{
		"action": "opened",
		"repository": {"full_name": "org/repo"},
		"pull_request": {
			"number": 99,
			"title": "Add new feature",
			"head": {"ref": "feature-branch", "sha": "deadbeef"},
			"base": {"ref": "main"},
			"user": {"login": "bob"},
			"created_at": "2025-03-10T14:00:00Z"
		}
	}`
	body := []byte(payload)
	sig := signPayload(secret, body)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sig)
	req.Header.Set("X-GitHub-Event", "pull_request")

	event, err := ParseGitHubWebhook(req, secret)
	if err != nil {
		t.Fatalf("ParseGitHubWebhook returned error: %v", err)
	}
	if event.Type != "pull_request" {
		t.Errorf("Type = %q, want %q", event.Type, "pull_request")
	}
	if event.Repository != "org/repo" {
		t.Errorf("Repository = %q, want %q", event.Repository, "org/repo")
	}
	if event.Branch != "feature-branch" {
		t.Errorf("Branch = %q, want %q", event.Branch, "feature-branch")
	}
	if event.CommitHash != "deadbeef" {
		t.Errorf("CommitHash = %q, want %q", event.CommitHash, "deadbeef")
	}
	if event.Author != "bob" {
		t.Errorf("Author = %q, want %q", event.Author, "bob")
	}
	if event.PullRequest == nil {
		t.Fatal("PullRequest should not be nil")
	}
	if event.PullRequest.Number != 99 {
		t.Errorf("PR Number = %d, want 99", event.PullRequest.Number)
	}
	if event.PullRequest.Title != "Add new feature" {
		t.Errorf("PR Title = %q, want %q", event.PullRequest.Title, "Add new feature")
	}
	if event.PullRequest.SourceBranch != "feature-branch" {
		t.Errorf("SourceBranch = %q, want %q", event.PullRequest.SourceBranch, "feature-branch")
	}
	if event.PullRequest.TargetBranch != "main" {
		t.Errorf("TargetBranch = %q, want %q", event.PullRequest.TargetBranch, "main")
	}
	if event.PullRequest.Action != "opened" {
		t.Errorf("Action = %q, want %q", event.PullRequest.Action, "opened")
	}
}

func TestVerifySignature(t *testing.T) {
	secret := "my-secret"
	payload := []byte(`{"ref":"refs/heads/main"}`)

	tests := []struct {
		name    string
		sigHdr  string
		wantErr error
	}{
		{
			name:    "missing signature header",
			sigHdr:  "",
			wantErr: ErrMissingSignature,
		},
		{
			name:    "invalid prefix",
			sigHdr:  "md5=abc",
			wantErr: ErrInvalidSignature,
		},
		{
			name:    "wrong signature value",
			sigHdr:  "sha256=0000000000000000000000000000000000000000000000000000000000000000",
			wantErr: ErrInvalidSignature,
		},
		{
			name:    "valid signature",
			sigHdr:  signPayload(secret, payload),
			wantErr: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(payload))
			req.Header.Set("X-Hub-Signature-256", tc.sigHdr)
			req.Header.Set("X-GitHub-Event", "push")

			_, err := ParseGitHubWebhook(req, secret)

			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error %v, got nil", tc.wantErr)
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestServeHTTP_RoutingByEvent(t *testing.T) {
	secret := "route-secret"

	tests := []struct {
		name         string
		method       string
		ghEvent      string
		glEvent      string
		glToken      string
		body         string
		wantStatus   int
		wantContains string
	}{
		{
			name:         "GET method rejected",
			method:       http.MethodGet,
			body:         "",
			wantStatus:   http.StatusMethodNotAllowed,
			wantContains: "method not allowed",
		},
		{
			name:         "no provider headers",
			method:       http.MethodPost,
			body:         `{}`,
			wantStatus:   http.StatusBadRequest,
			wantContains: "unsupported webhook provider",
		},
		{
			name:         "github push success",
			method:       http.MethodPost,
			ghEvent:      "push",
			body:         `{"ref":"refs/heads/main","repository":{"full_name":"o/r"},"head_commit":{"id":"aaa","message":"m","author":{"name":"a"},"timestamp":"2025-01-01T00:00:00Z"}}`,
			wantStatus:   http.StatusOK,
			wantContains: `"status":"ok"`,
		},
		{
			name:         "github unsupported event type",
			method:       http.MethodPost,
			ghEvent:      "release",
			body:         `{"action":"published"}`,
			wantStatus:   http.StatusBadRequest,
			wantContains: "unsupported event",
		},
		{
			name:         "gitlab push success",
			method:       http.MethodPost,
			glEvent:      "Push Hook",
			glToken:      secret,
			body:         `{"ref":"refs/heads/main","project":{"path_with_namespace":"g/r"},"commits":[{"id":"bbb","message":"m","author":{"name":"a"},"timestamp":"2025-01-01T00:00:00+00:00"}],"user_name":"u"}`,
			wantStatus:   http.StatusOK,
			wantContains: `"status":"ok"`,
		},
		{
			name:         "gitlab invalid token",
			method:       http.MethodPost,
			glEvent:      "Push Hook",
			glToken:      "wrong-token",
			body:         `{"ref":"refs/heads/main","project":{"path_with_namespace":"g/r"},"commits":[],"user_name":"u"}`,
			wantStatus:   http.StatusUnauthorized,
			wantContains: "unauthorized",
		},
		{
			name:         "gitlab missing token",
			method:       http.MethodPost,
			glEvent:      "Push Hook",
			glToken:      "",
			body:         `{"ref":"refs/heads/main","project":{"path_with_namespace":"g/r"},"commits":[],"user_name":"u"}`,
			wantStatus:   http.StatusUnauthorized,
			wantContains: "missing authentication",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var receivedEvent *WebhookEvent
			handler := NewWebhookHandler(secret, func(e *WebhookEvent) {
				receivedEvent = e
			})

			body := []byte(tc.body)
			req := httptest.NewRequest(tc.method, "/webhook", bytes.NewReader(body))

			if tc.ghEvent != "" {
				req.Header.Set("X-GitHub-Event", tc.ghEvent)
				req.Header.Set("X-Hub-Signature-256", signPayload(secret, body))
			}
			if tc.glEvent != "" {
				req.Header.Set("X-Gitlab-Event", tc.glEvent)
				if tc.glToken != "" {
					req.Header.Set("X-Gitlab-Token", tc.glToken)
				}
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d; body = %s", rr.Code, tc.wantStatus, rr.Body.String())
			}
			if tc.wantContains != "" && !strings.Contains(rr.Body.String(), tc.wantContains) {
				t.Errorf("body = %q, want substring %q", rr.Body.String(), tc.wantContains)
			}
			if tc.wantStatus == http.StatusOK && receivedEvent == nil {
				t.Error("callback was not invoked on success")
			}
		})
	}
}

func TestParseGitLabWebhook_MergeRequest(t *testing.T) {
	secret := "gl-secret"
	payload := `{
		"project": {"path_with_namespace": "group/project"},
		"object_attributes": {
			"iid": 15,
			"title": "Update docs",
			"source_branch": "docs-update",
			"target_branch": "main",
			"last_commit": {"id": "cafe1234"},
			"action": "open",
			"created_at": "2025-03-10 12:00:00 UTC"
		},
		"user": {"name": "Charlie"}
	}`
	body := []byte(payload)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Gitlab-Token", secret)
	req.Header.Set("X-Gitlab-Event", "Merge Request Hook")

	event, err := ParseGitLabWebhook(req, secret)
	if err != nil {
		t.Fatalf("ParseGitLabWebhook returned error: %v", err)
	}
	if event.Type != "merge_request" {
		t.Errorf("Type = %q, want %q", event.Type, "merge_request")
	}
	if event.Repository != "group/project" {
		t.Errorf("Repository = %q, want %q", event.Repository, "group/project")
	}
	if event.Branch != "docs-update" {
		t.Errorf("Branch = %q, want %q", event.Branch, "docs-update")
	}
	if event.CommitHash != "cafe1234" {
		t.Errorf("CommitHash = %q, want %q", event.CommitHash, "cafe1234")
	}
	if event.Author != "Charlie" {
		t.Errorf("Author = %q, want %q", event.Author, "Charlie")
	}
	if event.PullRequest == nil {
		t.Fatal("PullRequest should not be nil")
	}
	if event.PullRequest.Number != 15 {
		t.Errorf("PR Number = %d, want 15", event.PullRequest.Number)
	}
	if event.PullRequest.SourceBranch != "docs-update" {
		t.Errorf("SourceBranch = %q, want %q", event.PullRequest.SourceBranch, "docs-update")
	}
	if event.PullRequest.TargetBranch != "main" {
		t.Errorf("TargetBranch = %q, want %q", event.PullRequest.TargetBranch, "main")
	}
	if event.PullRequest.Action != "open" {
		t.Errorf("Action = %q, want %q", event.PullRequest.Action, "open")
	}
}

func TestParseGitHubWebhook_EmptyBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader([]byte{}))
	req.Header.Set("X-Hub-Signature-256", "sha256=abc")
	req.Header.Set("X-GitHub-Event", "push")

	_, err := ParseGitHubWebhook(req, "secret")
	if err == nil {
		t.Fatal("expected error for empty body, got nil")
	}
	if !errors.Is(err, ErrEmptyBody) {
		t.Errorf("error = %v, want %v", err, ErrEmptyBody)
	}
}

func TestServeHTTP_CallbackReceivesCorrectEvent(t *testing.T) {
	secret := "cb-secret"
	payload := map[string]interface{}{
		"ref":         "refs/heads/develop",
		"repository":  map[string]string{"full_name": "myorg/myrepo"},
		"head_commit": map[string]interface{}{"id": "abc123", "message": "test commit", "author": map[string]string{"name": "Dev"}, "timestamp": "2025-06-01T10:00:00Z"},
	}
	body, _ := json.Marshal(payload)

	var receivedEvent *WebhookEvent
	handler := NewWebhookHandler(secret, func(e *WebhookEvent) {
		receivedEvent = e
	})

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", signPayload(secret, body))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if receivedEvent == nil {
		t.Fatal("callback was not invoked")
	}
	if receivedEvent.Type != "push" {
		t.Errorf("event.Type = %q, want %q", receivedEvent.Type, "push")
	}
	if receivedEvent.Repository != "myorg/myrepo" {
		t.Errorf("event.Repository = %q, want %q", receivedEvent.Repository, "myorg/myrepo")
	}
	if receivedEvent.Branch != "develop" {
		t.Errorf("event.Branch = %q, want %q", receivedEvent.Branch, "develop")
	}
}
