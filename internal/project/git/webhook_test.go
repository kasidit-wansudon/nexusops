package git

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func signPayload(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestParseGitHubWebhookPush(t *testing.T) {
	secret := "test-secret"
	payload := map[string]interface{}{
		"ref": "refs/heads/main",
		"repository": map[string]interface{}{
			"full_name": "org/repo",
		},
		"head_commit": map[string]interface{}{
			"id":      "abc123",
			"message": "fix: bug",
			"author":  map[string]string{"name": "Dev"},
			"timestamp": "2024-01-15T10:30:00Z",
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", signPayload(secret, body))

	event, err := ParseGitHubWebhook(req, secret)
	require.NoError(t, err)
	assert.Equal(t, "push", event.Type)
	assert.Equal(t, "org/repo", event.Repository)
	assert.Equal(t, "main", event.Branch)
	assert.Equal(t, "abc123", event.CommitHash)
	assert.Equal(t, "fix: bug", event.CommitMessage)
	assert.Equal(t, "Dev", event.Author)
	assert.Nil(t, event.PullRequest)
}

func TestParseGitHubWebhookPullRequest(t *testing.T) {
	secret := "test-secret"
	payload := map[string]interface{}{
		"action": "opened",
		"repository": map[string]interface{}{
			"full_name": "org/repo",
		},
		"pull_request": map[string]interface{}{
			"number":     42,
			"title":      "Add feature",
			"head":       map[string]string{"ref": "feature-branch", "sha": "def456"},
			"base":       map[string]string{"ref": "main"},
			"user":       map[string]string{"login": "dev-user"},
			"created_at": "2024-01-15T10:30:00Z",
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-Hub-Signature-256", signPayload(secret, body))

	event, err := ParseGitHubWebhook(req, secret)
	require.NoError(t, err)
	assert.Equal(t, "pull_request", event.Type)
	assert.Equal(t, "feature-branch", event.Branch)
	require.NotNil(t, event.PullRequest)
	assert.Equal(t, 42, event.PullRequest.Number)
	assert.Equal(t, "Add feature", event.PullRequest.Title)
	assert.Equal(t, "opened", event.PullRequest.Action)
}

func TestParseGitHubWebhookInvalidSignature(t *testing.T) {
	payload := []byte(`{"ref":"refs/heads/main"}`)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(payload))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", "sha256=invalid")

	_, err := ParseGitHubWebhook(req, "real-secret")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidSignature)
}

func TestParseGitHubWebhookMissingSignature(t *testing.T) {
	payload := []byte(`{"ref":"refs/heads/main"}`)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(payload))
	req.Header.Set("X-GitHub-Event", "push")
	// No signature header

	_, err := ParseGitHubWebhook(req, "secret")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMissingSignature)
}

func TestParseGitLabWebhookPush(t *testing.T) {
	secret := "gitlab-token"
	payload := map[string]interface{}{
		"ref": "refs/heads/develop",
		"project": map[string]interface{}{
			"path_with_namespace": "team/project",
		},
		"commits": []map[string]interface{}{
			{
				"id":      "commit1",
				"message": "first commit",
				"author":  map[string]string{"name": "Author1"},
				"timestamp": "2024-01-15T10:30:00+00:00",
			},
			{
				"id":      "commit2",
				"message": "second commit",
				"author":  map[string]string{"name": "Author2"},
				"timestamp": "2024-01-15T11:00:00+00:00",
			},
		},
		"user_name": "pusher",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Gitlab-Event", "Push Hook")
	req.Header.Set("X-Gitlab-Token", secret)

	event, err := ParseGitLabWebhook(req, secret)
	require.NoError(t, err)
	assert.Equal(t, "push", event.Type)
	assert.Equal(t, "team/project", event.Repository)
	assert.Equal(t, "develop", event.Branch)
	// Should use the last commit
	assert.Equal(t, "commit2", event.CommitHash)
	assert.Equal(t, "Author2", event.Author)
}

func TestParseGitLabWebhookInvalidToken(t *testing.T) {
	payload := []byte(`{"ref":"refs/heads/main"}`)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(payload))
	req.Header.Set("X-Gitlab-Event", "Push Hook")
	req.Header.Set("X-Gitlab-Token", "wrong-token")

	_, err := ParseGitLabWebhook(req, "correct-token")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestWebhookHandlerAutoDetect(t *testing.T) {
	secret := "handler-secret"

	var received *WebhookEvent
	handler := NewWebhookHandler(secret, func(e *WebhookEvent) {
		received = e
	})

	// Test GitHub webhook
	payload := map[string]interface{}{
		"ref": "refs/heads/main",
		"repository": map[string]interface{}{
			"full_name": "org/repo",
		},
		"head_commit": map[string]interface{}{
			"id":        "abc123",
			"message":   "test",
			"author":    map[string]string{"name": "Dev"},
			"timestamp": "2024-01-15T10:30:00Z",
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", signPayload(secret, body))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, received)
	assert.Equal(t, "push", received.Type)
	assert.Equal(t, "org/repo", received.Repository)
}

func TestWebhookHandlerMethodNotAllowed(t *testing.T) {
	handler := NewWebhookHandler("secret", nil)

	req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestWebhookHandlerUnsupportedProvider(t *testing.T) {
	handler := NewWebhookHandler("secret", nil)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader([]byte(`{}`)))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetCommitDiffValidation(t *testing.T) {
	// Empty repo path
	_, err := GetCommitDiff("", "abc123")
	assert.ErrorIs(t, err, ErrInvalidRepoPath)

	// Empty commit hash
	_, err = GetCommitDiff("/some/path", "")
	assert.ErrorIs(t, err, ErrInvalidCommitHash)

	// Invalid characters in commit hash
	_, err = GetCommitDiff("/some/path", "abc;rm -rf /")
	assert.ErrorIs(t, err, ErrInvalidCommitHash)
}
