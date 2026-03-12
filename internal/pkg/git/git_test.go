package git

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestParseGitHubWebhookPush(t *testing.T) {
	payload := map[string]interface{}{
		"ref":   "refs/heads/main",
		"after": "abc123def456",
		"repository": map[string]interface{}{
			"full_name": "myorg/myrepo",
			"clone_url": "https://github.com/myorg/myrepo.git",
		},
		"pusher": map[string]interface{}{
			"name": "johndoe",
		},
		"head_commit": map[string]interface{}{
			"message": "feat: add new feature",
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	event, err := ParseGitHubWebhook(data, "")
	if err != nil {
		t.Fatalf("ParseGitHubWebhook failed: %v", err)
	}
	if event == nil {
		t.Fatal("ParseGitHubWebhook returned nil event")
	}
	if event.Type != EventPush {
		t.Errorf("Type = %q, want %q", event.Type, EventPush)
	}
	if event.Repo != "myorg/myrepo" {
		t.Errorf("Repo = %q, want %q", event.Repo, "myorg/myrepo")
	}
	if event.Branch != "main" {
		t.Errorf("Branch = %q, want %q", event.Branch, "main")
	}
	if event.CommitHash != "abc123def456" {
		t.Errorf("CommitHash = %q, want %q", event.CommitHash, "abc123def456")
	}
	if event.Author != "johndoe" {
		t.Errorf("Author = %q, want %q", event.Author, "johndoe")
	}
	if event.Message != "feat: add new feature" {
		t.Errorf("Message = %q, want %q", event.Message, "feat: add new feature")
	}
	if event.Timestamp.IsZero() {
		t.Error("Timestamp is zero")
	}
}

func TestParseGitHubWebhookTag(t *testing.T) {
	payload := map[string]interface{}{
		"ref":   "refs/tags/v1.0.0",
		"after": "def789abc012",
		"repository": map[string]interface{}{
			"full_name": "myorg/myrepo",
			"clone_url": "https://github.com/myorg/myrepo.git",
		},
		"pusher": map[string]interface{}{
			"name": "releasebot",
		},
		"head_commit": map[string]interface{}{
			"message": "Release v1.0.0",
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	event, err := ParseGitHubWebhook(data, "")
	if err != nil {
		t.Fatalf("ParseGitHubWebhook failed: %v", err)
	}
	if event.Type != EventTag {
		t.Errorf("Type = %q, want %q", event.Type, EventTag)
	}
	if event.Branch != "v1.0.0" {
		t.Errorf("Branch = %q, want %q", event.Branch, "v1.0.0")
	}
}

func TestParseGitHubWebhookPullRequest(t *testing.T) {
	payload := map[string]interface{}{
		"action": "opened",
		"number": 42,
		"pull_request": map[string]interface{}{
			"title": "Add amazing feature",
			"head": map[string]interface{}{
				"ref": "feature-branch",
				"sha": "aabbccdd",
			},
			"base": map[string]interface{}{
				"ref": "main",
			},
			"html_url": "https://github.com/myorg/myrepo/pull/42",
			"user": map[string]interface{}{
				"login": "contributor",
			},
		},
		"repository": map[string]interface{}{
			"full_name": "myorg/myrepo",
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	event, err := ParseGitHubWebhook(data, "")
	if err != nil {
		t.Fatalf("ParseGitHubWebhook failed: %v", err)
	}
	if event.Type != EventPullRequest {
		t.Errorf("Type = %q, want %q", event.Type, EventPullRequest)
	}
	if event.Branch != "feature-branch" {
		t.Errorf("Branch = %q, want %q", event.Branch, "feature-branch")
	}
	if event.Author != "contributor" {
		t.Errorf("Author = %q, want %q", event.Author, "contributor")
	}
	if event.PullRequest == nil {
		t.Fatal("PullRequest is nil")
	}
	if event.PullRequest.Number != 42 {
		t.Errorf("PullRequest.Number = %d, want 42", event.PullRequest.Number)
	}
	if event.PullRequest.Title != "Add amazing feature" {
		t.Errorf("PullRequest.Title = %q, want %q", event.PullRequest.Title, "Add amazing feature")
	}
	if event.PullRequest.Action != "opened" {
		t.Errorf("PullRequest.Action = %q, want %q", event.PullRequest.Action, "opened")
	}
	if event.PullRequest.SourceBranch != "feature-branch" {
		t.Errorf("PullRequest.SourceBranch = %q, want %q", event.PullRequest.SourceBranch, "feature-branch")
	}
	if event.PullRequest.TargetBranch != "main" {
		t.Errorf("PullRequest.TargetBranch = %q, want %q", event.PullRequest.TargetBranch, "main")
	}
}

func TestParseGitHubWebhookInvalid(t *testing.T) {
	_, err := ParseGitHubWebhook([]byte(`{}`), "")
	if err == nil {
		t.Fatal("expected error for empty payload, got nil")
	}
}

func TestParseGitLabWebhookPush(t *testing.T) {
	payload := map[string]interface{}{
		"ref":       "refs/heads/develop",
		"after":     "789abc012def",
		"user_name": "janedoe",
		"project": map[string]interface{}{
			"path_with_namespace": "mygroup/myproject",
		},
		"commits": []map[string]interface{}{
			{
				"message": "fix: resolve issue #42",
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	event, err := ParseGitLabWebhook(data, "")
	if err != nil {
		t.Fatalf("ParseGitLabWebhook failed: %v", err)
	}
	if event.Type != EventPush {
		t.Errorf("Type = %q, want %q", event.Type, EventPush)
	}
	if event.Repo != "mygroup/myproject" {
		t.Errorf("Repo = %q, want %q", event.Repo, "mygroup/myproject")
	}
	if event.Branch != "develop" {
		t.Errorf("Branch = %q, want %q", event.Branch, "develop")
	}
	if event.CommitHash != "789abc012def" {
		t.Errorf("CommitHash = %q, want %q", event.CommitHash, "789abc012def")
	}
	if event.Author != "janedoe" {
		t.Errorf("Author = %q, want %q", event.Author, "janedoe")
	}
	if event.Message != "fix: resolve issue #42" {
		t.Errorf("Message = %q, want %q", event.Message, "fix: resolve issue #42")
	}
}

func TestParseGitLabWebhookInvalid(t *testing.T) {
	_, err := ParseGitLabWebhook([]byte(`{}`), "")
	if err == nil {
		t.Fatal("expected error for empty payload, got nil")
	}
}

func TestVerifyGitHubSignature(t *testing.T) {
	payload := []byte(`{"test":"payload"}`)
	secret := "my-webhook-secret"

	// Compute valid signature.
	validSig := computeTestHMACSHA256(payload, secret)

	tests := []struct {
		name      string
		payload   []byte
		signature string
		secret    string
		want      bool
	}{
		{"valid signature", payload, validSig, secret, true},
		{"wrong signature", payload, "sha256=0000000000000000000000000000000000000000000000000000000000000000", secret, false},
		{"wrong secret", payload, validSig, "wrong-secret", false},
		{"empty signature", payload, "", secret, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := VerifyGitHubSignature(tc.payload, tc.signature, tc.secret)
			if got != tc.want {
				t.Errorf("VerifyGitHubSignature = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestExtractBranch(t *testing.T) {
	// Test branch extraction through the webhook parser, which strips refs/heads/.
	tests := []struct {
		name       string
		ref        string
		wantBranch string
		wantType   WebhookEventType
	}{
		{"main branch", "refs/heads/main", "main", EventPush},
		{"feature branch", "refs/heads/feature/my-feature", "feature/my-feature", EventPush},
		{"tag ref", "refs/tags/v1.0.0", "v1.0.0", EventTag},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload := map[string]interface{}{
				"ref":   tc.ref,
				"after": "deadbeef",
				"repository": map[string]interface{}{
					"full_name": "org/repo",
					"clone_url": "https://github.com/org/repo.git",
				},
				"pusher": map[string]interface{}{
					"name": "user",
				},
				"head_commit": map[string]interface{}{
					"message": "commit msg",
				},
			}
			data, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("json.Marshal failed: %v", err)
			}

			event, err := ParseGitHubWebhook(data, "")
			if err != nil {
				t.Fatalf("ParseGitHubWebhook failed: %v", err)
			}
			if event.Branch != tc.wantBranch {
				t.Errorf("Branch = %q, want %q", event.Branch, tc.wantBranch)
			}
			if event.Type != tc.wantType {
				t.Errorf("Type = %q, want %q", event.Type, tc.wantType)
			}
		})
	}
}

func TestInjectToken(t *testing.T) {
	tests := []struct {
		name  string
		url   string
		token string
		want  string
	}{
		{"no token", "https://github.com/org/repo.git", "", "https://github.com/org/repo.git"},
		{"with token", "https://github.com/org/repo.git", "ghp_abc123", "https://ghp_abc123@github.com/org/repo.git"},
		{"ssh url unchanged", "git@github.com:org/repo.git", "ghp_abc123", "git@github.com:org/repo.git"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := injectToken(tc.url, tc.token)
			if got != tc.want {
				t.Errorf("injectToken(%q, %q) = %q, want %q", tc.url, tc.token, got, tc.want)
			}
		})
	}
}

func TestWebhookEventTypes(t *testing.T) {
	if string(EventPush) != "push" {
		t.Errorf("EventPush = %q, want %q", EventPush, "push")
	}
	if string(EventPullRequest) != "pull_request" {
		t.Errorf("EventPullRequest = %q, want %q", EventPullRequest, "pull_request")
	}
	if string(EventTag) != "tag" {
		t.Errorf("EventTag = %q, want %q", EventTag, "tag")
	}
	if string(EventUnknown) != "unknown" {
		t.Errorf("EventUnknown = %q, want %q", EventUnknown, "unknown")
	}
}

// computeTestHMACSHA256 is a test helper that produces the "sha256=..." signature.
func computeTestHMACSHA256(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
