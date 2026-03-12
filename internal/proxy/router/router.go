package router

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Route represents a subdomain-to-container routing rule.
type Route struct {
	Subdomain   string    `json:"subdomain"`
	Target      string    `json:"target"` // host:port of the backend container
	ProjectID   string    `json:"project_id"`
	Environment string    `json:"environment"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// subdomainPattern validates subdomain format: lowercase alphanumeric with optional hyphens,
// no leading/trailing hyphens, max 63 characters per label.
var subdomainPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9\-]{0,61}[a-z0-9])?$`)

// Router manages dynamic subdomain-to-container routing and implements http.Handler
// for reverse proxying requests to the appropriate backend.
type Router struct {
	routes        map[string]*Route
	mu            sync.RWMutex
	defaultTarget string
}

// NewRouter creates a new Router with the given default target for unmatched subdomains.
// The defaultTarget should be in host:port format (e.g., "localhost:8080").
func NewRouter(defaultTarget string) *Router {
	return &Router{
		routes:        make(map[string]*Route),
		defaultTarget: defaultTarget,
	}
}

// AddRoute registers a new subdomain route. It validates the subdomain format
// and returns an error if the format is invalid or the subdomain already exists.
func (r *Router) AddRoute(subdomain, target, projectID, env string) error {
	subdomain = strings.ToLower(strings.TrimSpace(subdomain))
	if subdomain == "" {
		return fmt.Errorf("subdomain cannot be empty")
	}
	if !subdomainPattern.MatchString(subdomain) {
		return fmt.Errorf("invalid subdomain format %q: must be lowercase alphanumeric with optional hyphens, 1-63 chars", subdomain)
	}
	if target == "" {
		return fmt.Errorf("target cannot be empty")
	}
	if !strings.Contains(target, ":") {
		return fmt.Errorf("target %q must be in host:port format", target)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.routes[subdomain]; exists {
		return fmt.Errorf("route for subdomain %q already exists", subdomain)
	}

	now := time.Now().UTC()
	r.routes[subdomain] = &Route{
		Subdomain:   subdomain,
		Target:      target,
		ProjectID:   projectID,
		Environment: env,
		Active:      true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	return nil
}

// RemoveRoute deletes the route for the specified subdomain.
func (r *Router) RemoveRoute(subdomain string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.routes, strings.ToLower(strings.TrimSpace(subdomain)))
}

// UpdateRoute changes the target backend for an existing subdomain route.
// Returns an error if the subdomain route does not exist or the new target is empty.
func (r *Router) UpdateRoute(subdomain, newTarget string) error {
	subdomain = strings.ToLower(strings.TrimSpace(subdomain))
	if newTarget == "" {
		return fmt.Errorf("new target cannot be empty")
	}
	if !strings.Contains(newTarget, ":") {
		return fmt.Errorf("target %q must be in host:port format", newTarget)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	route, exists := r.routes[subdomain]
	if !exists {
		return fmt.Errorf("route for subdomain %q not found", subdomain)
	}

	route.Target = newTarget
	route.UpdatedAt = time.Now().UTC()
	return nil
}

// GetRoute returns the route associated with the given subdomain.
func (r *Router) GetRoute(subdomain string) (*Route, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	route, exists := r.routes[strings.ToLower(strings.TrimSpace(subdomain))]
	if !exists {
		return nil, fmt.Errorf("route for subdomain %q not found", subdomain)
	}
	return route, nil
}

// ListRoutes returns all registered routes sorted by subdomain name.
func (r *Router) ListRoutes() []*Route {
	r.mu.RLock()
	defer r.mu.RUnlock()

	routes := make([]*Route, 0, len(r.routes))
	for _, route := range r.routes {
		routes = append(routes, route)
	}
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].Subdomain < routes[j].Subdomain
	})
	return routes
}

// extractSubdomain pulls the first subdomain label from a host string.
// For example, "app.example.com" yields "app", and "example.com" yields "".
func extractSubdomain(host string) string {
	// Strip port if present
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		// Only strip if it looks like a port (after last colon)
		possiblePort := host[idx+1:]
		if _, err := fmt.Sscanf(possiblePort, "%d", new(int)); err == nil {
			host = host[:idx]
		}
	}

	parts := strings.Split(host, ".")
	if len(parts) < 3 {
		// Not enough parts for a subdomain (e.g., "example.com")
		return ""
	}
	return strings.ToLower(parts[0])
}

// MatchRoute extracts the subdomain from the given host and looks up the
// corresponding route. Returns an error if no matching route is found.
func (r *Router) MatchRoute(host string) (*Route, error) {
	subdomain := extractSubdomain(host)
	if subdomain == "" {
		return nil, fmt.Errorf("no subdomain found in host %q", host)
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Direct match first
	if route, exists := r.routes[subdomain]; exists && route.Active {
		return route, nil
	}

	// Try wildcard patterns
	for pattern, route := range r.routes {
		if route.Active && WildcardMatch(pattern, subdomain) {
			return route, nil
		}
	}

	return nil, fmt.Errorf("no route found for subdomain %q", subdomain)
}

// WildcardMatch checks whether a pattern (which may contain '*' wildcards)
// matches the given host string. Each '*' matches zero or more characters.
func WildcardMatch(pattern, host string) bool {
	if pattern == "*" {
		return true
	}
	if !strings.Contains(pattern, "*") {
		return pattern == host
	}

	// Simple wildcard matching using dynamic programming
	pLen := len(pattern)
	hLen := len(host)

	// dp[i][j] means pattern[:i] matches host[:j]
	dp := make([][]bool, pLen+1)
	for i := range dp {
		dp[i] = make([]bool, hLen+1)
	}
	dp[0][0] = true

	// Handle leading wildcards
	for i := 1; i <= pLen; i++ {
		if pattern[i-1] == '*' {
			dp[i][0] = dp[i-1][0]
		}
	}

	for i := 1; i <= pLen; i++ {
		for j := 1; j <= hLen; j++ {
			if pattern[i-1] == '*' {
				dp[i][j] = dp[i-1][j] || dp[i][j-1]
			} else if pattern[i-1] == host[j-1] || pattern[i-1] == '?' {
				dp[i][j] = dp[i-1][j-1]
			}
		}
	}

	return dp[pLen][hLen]
}

// ServeHTTP implements http.Handler. It routes incoming requests based on the
// subdomain extracted from the Host header, reverse-proxying to the matched
// backend target. If no route matches, the default target is used.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	route, err := r.MatchRoute(req.Host)

	var targetAddr string
	if err != nil || route == nil {
		if r.defaultTarget == "" {
			http.Error(w, "no route found and no default target configured", http.StatusBadGateway)
			return
		}
		targetAddr = r.defaultTarget
	} else {
		targetAddr = route.Target
	}

	targetURL, err := url.Parse("http://" + targetAddr)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid target URL: %v", err), http.StatusInternalServerError)
		return
	}

	proxy := &httputil.ReverseProxy{
		Director: func(outReq *http.Request) {
			outReq.URL.Scheme = targetURL.Scheme
			outReq.URL.Host = targetURL.Host
			outReq.URL.Path = singleJoiningSlash(targetURL.Path, outReq.URL.Path)
			outReq.Host = targetURL.Host

			// Preserve original headers
			if _, ok := outReq.Header["User-Agent"]; !ok {
				outReq.Header.Set("User-Agent", "")
			}
			// Set forwarding headers
			outReq.Header.Set("X-Forwarded-Host", req.Host)
			outReq.Header.Set("X-Forwarded-Proto", schemeFromRequest(req))
			if clientIP := req.RemoteAddr; clientIP != "" {
				if prior := outReq.Header.Get("X-Forwarded-For"); prior != "" {
					outReq.Header.Set("X-Forwarded-For", prior+", "+stripPort(clientIP))
				} else {
					outReq.Header.Set("X-Forwarded-For", stripPort(clientIP))
				}
			}
			if route != nil {
				outReq.Header.Set("X-NexusOps-Project", route.ProjectID)
				outReq.Header.Set("X-NexusOps-Environment", route.Environment)
			}
		},
		ErrorHandler: func(rw http.ResponseWriter, req *http.Request, err error) {
			http.Error(rw, fmt.Sprintf("proxy error: %v", err), http.StatusBadGateway)
		},
	}

	proxy.ServeHTTP(w, req)
}

// singleJoiningSlash joins two URL path segments with exactly one slash.
func singleJoiningSlash(a, b string) string {
	aSlash := strings.HasSuffix(a, "/")
	bSlash := strings.HasPrefix(b, "/")
	switch {
	case aSlash && bSlash:
		return a + b[1:]
	case !aSlash && !bSlash:
		return a + "/" + b
	}
	return a + b
}

// schemeFromRequest determines the scheme (http or https) from the request.
func schemeFromRequest(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		return proto
	}
	return "http"
}

// stripPort removes the port component from an address string.
func stripPort(addr string) string {
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}
