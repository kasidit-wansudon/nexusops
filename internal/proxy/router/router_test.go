package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddRouteAndGetRoute(t *testing.T) {
	r := NewRouter("localhost:9090")

	err := r.AddRoute("myapp", "localhost:8080", "proj-1", "production")
	require.NoError(t, err)

	route, err := r.GetRoute("myapp")
	require.NoError(t, err)
	assert.Equal(t, "myapp", route.Subdomain)
	assert.Equal(t, "localhost:8080", route.Target)
	assert.Equal(t, "proj-1", route.ProjectID)
	assert.Equal(t, "production", route.Environment)
	assert.True(t, route.Active)
	assert.False(t, route.CreatedAt.IsZero())
}

func TestAddRouteValidation(t *testing.T) {
	r := NewRouter("localhost:9090")

	tests := []struct {
		name      string
		subdomain string
		target    string
		errMsg    string
	}{
		{"empty subdomain", "", "localhost:8080", "subdomain cannot be empty"},
		{"invalid subdomain format", "MY_APP!", "localhost:8080", "invalid subdomain format"},
		{"empty target", "myapp", "", "target cannot be empty"},
		{"target without port", "myapp", "localhost", "must be in host:port format"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := r.AddRoute(tc.subdomain, tc.target, "proj-1", "dev")
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errMsg)
		})
	}
}

func TestAddRouteDuplicate(t *testing.T) {
	r := NewRouter("localhost:9090")

	err := r.AddRoute("myapp", "localhost:8080", "proj-1", "dev")
	require.NoError(t, err)

	err = r.AddRoute("myapp", "localhost:9090", "proj-2", "staging")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestRemoveRoute(t *testing.T) {
	r := NewRouter("localhost:9090")

	err := r.AddRoute("myapp", "localhost:8080", "proj-1", "dev")
	require.NoError(t, err)

	r.RemoveRoute("myapp")

	_, err = r.GetRoute("myapp")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUpdateRoute(t *testing.T) {
	r := NewRouter("localhost:9090")

	err := r.AddRoute("myapp", "localhost:8080", "proj-1", "dev")
	require.NoError(t, err)

	err = r.UpdateRoute("myapp", "localhost:9999")
	require.NoError(t, err)

	route, err := r.GetRoute("myapp")
	require.NoError(t, err)
	assert.Equal(t, "localhost:9999", route.Target)

	// Update non-existent
	err = r.UpdateRoute("nonexistent", "localhost:1234")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Update with empty target
	err = r.UpdateRoute("myapp", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestListRoutes(t *testing.T) {
	r := NewRouter("localhost:9090")

	_ = r.AddRoute("beta", "localhost:8081", "proj-1", "dev")
	_ = r.AddRoute("alpha", "localhost:8082", "proj-2", "staging")

	routes := r.ListRoutes()
	require.Len(t, routes, 2)
	// Should be sorted by subdomain
	assert.Equal(t, "alpha", routes[0].Subdomain)
	assert.Equal(t, "beta", routes[1].Subdomain)
}

func TestWildcardMatch(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		host     string
		expected bool
	}{
		{"exact match", "myapp", "myapp", true},
		{"no match", "myapp", "other", false},
		{"star matches all", "*", "anything", true},
		{"prefix wildcard", "*-api", "myapp-api", true},
		{"suffix wildcard", "app-*", "app-v2", true},
		{"middle wildcard", "app-*-v2", "app-staging-v2", true},
		{"empty host star", "*", "", true},
		{"no wildcard mismatch", "abc", "xyz", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := WildcardMatch(tc.pattern, tc.host)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestMatchRoute(t *testing.T) {
	r := NewRouter("localhost:9090")
	_ = r.AddRoute("myapp", "localhost:8080", "proj-1", "production")

	// Successful match
	route, err := r.MatchRoute("myapp.example.com")
	require.NoError(t, err)
	assert.Equal(t, "myapp", route.Subdomain)

	// Match with port in host
	route, err = r.MatchRoute("myapp.example.com:443")
	require.NoError(t, err)
	assert.Equal(t, "myapp", route.Subdomain)

	// No subdomain
	_, err = r.MatchRoute("example.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no subdomain found")

	// Unknown subdomain
	_, err = r.MatchRoute("unknown.example.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no route found")
}

func TestServeHTTPDefaultTarget(t *testing.T) {
	r := NewRouter("")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "example.com"
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// No route found and no default target => bad gateway
	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.Contains(t, w.Body.String(), "no route found and no default target")
}
