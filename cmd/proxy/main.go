package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kasidit-wansudon/nexusops/internal/proxy/loadbalancer"
	"github.com/kasidit-wansudon/nexusops/internal/proxy/ratelimit"
	"github.com/kasidit-wansudon/nexusops/internal/proxy/router"
	proxyTLS "github.com/kasidit-wansudon/nexusops/internal/proxy/tls"
)

const (
	version    = "1.0.0"
	httpPort   = 80
	httpsPort  = 443
	adminPort  = 8082
)

type ProxyServer struct {
	router      *router.Router
	tlsManager  *proxyTLS.Manager
	pool        *loadbalancer.Pool
	rateLimiter *ratelimit.Limiter
}

func main() {
	fmt.Printf("NexusOps Reverse Proxy v%s\n", version)

	certDir := getEnv("TLS_CERT_DIR", "/tmp/nexusops/certs")
	email := getEnv("TLS_EMAIL", "admin@nexusops.dev")
	defaultTarget := getEnv("DEFAULT_TARGET", "localhost:8080")

	proxyRouter := router.NewRouter(defaultTarget)
	tlsManager := proxyTLS.NewManager(certDir, email)
	pool := loadbalancer.NewPool()
	limiter := ratelimit.NewLimiter(100, 200)

	proxy := &ProxyServer{
		router:      proxyRouter,
		tlsManager:  tlsManager,
		pool:        pool,
		rateLimiter: limiter,
	}

	if err := tlsManager.LoadCertificates(); err != nil {
		log.Printf("Warning: Could not load TLS certificates: %v", err)
	}

	pool.AddService("api", "round-robin")
	pool.AddService("frontend", "round-robin")

	setupDefaultRoutes(proxyRouter)

	handler := buildHandler(proxy)

	httpSrv := &http.Server{
		Addr:         fmt.Sprintf(":%d", httpPort),
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/health", proxy.adminHealth)
	adminMux.HandleFunc("/routes", proxy.adminRoutes)
	adminMux.HandleFunc("/routes/add", proxy.adminAddRoute)
	adminMux.HandleFunc("/routes/remove", proxy.adminRemoveRoute)
	adminMux.HandleFunc("/stats", proxy.adminStats)

	adminSrv := &http.Server{
		Addr:    fmt.Sprintf(":%d", adminPort),
		Handler: adminMux,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				limiter.Cleanup(1 * time.Hour)
			}
		}
	}()

	go func() {
		log.Printf("HTTP proxy listening on :%d", httpPort)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v (this is normal if port %d is unavailable)", err, httpPort)
		}
	}()

	go func() {
		log.Printf("Admin API listening on :%d", adminPort)
		if err := adminSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Admin server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Proxy shutting down...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	httpSrv.Shutdown(shutdownCtx)
	adminSrv.Shutdown(shutdownCtx)

	log.Println("Proxy stopped")
}

func buildHandler(proxy *ProxyServer) http.Handler {
	rateLimited := proxy.rateLimiter.Middleware(ratelimit.IPKeyFunc)(proxy.router)
	return rateLimited
}

func setupDefaultRoutes(r *router.Router) {
	r.AddRoute("api", "localhost:8080", "nexusops", "production")
	r.AddRoute("app", "localhost:3000", "nexusops", "production")
	r.AddRoute("monitor", "localhost:9090", "nexusops", "production")
}

func (p *ProxyServer) adminHealth(w http.ResponseWriter, r *http.Request) {
	routes := p.router.ListRoutes()
	fmt.Fprintf(w, `{"status":"healthy","routes":%d,"version":"%s"}`, len(routes), version)
}

func (p *ProxyServer) adminRoutes(w http.ResponseWriter, r *http.Request) {
	routes := p.router.ListRoutes()
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"routes":[`)
	for i, route := range routes {
		if i > 0 {
			fmt.Fprintf(w, ",")
		}
		fmt.Fprintf(w, `{"subdomain":"%s","target":"%s","project_id":"%s","active":%t}`,
			route.Subdomain, route.Target, route.ProjectID, route.Active)
	}
	fmt.Fprintf(w, `]}`)
}

func (p *ProxyServer) adminAddRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	subdomain := r.FormValue("subdomain")
	target := r.FormValue("target")
	projectID := r.FormValue("project_id")
	env := r.FormValue("environment")

	if subdomain == "" || target == "" {
		http.Error(w, "subdomain and target required", http.StatusBadRequest)
		return
	}

	if err := p.router.AddRoute(subdomain, target, projectID, env); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Fprintf(w, `{"status":"added","subdomain":"%s","target":"%s"}`, subdomain, target)
}

func (p *ProxyServer) adminRemoveRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	subdomain := r.FormValue("subdomain")
	if subdomain == "" {
		http.Error(w, "subdomain required", http.StatusBadRequest)
		return
	}
	p.router.RemoveRoute(subdomain)
	fmt.Fprintf(w, `{"status":"removed","subdomain":"%s"}`, subdomain)
}

func (p *ProxyServer) adminStats(w http.ResponseWriter, r *http.Request) {
	routes := p.router.ListRoutes()
	activeCount := 0
	for _, route := range routes {
		if route.Active {
			activeCount++
		}
	}
	fmt.Fprintf(w, `{"total_routes":%d,"active_routes":%d,"version":"%s"}`,
		len(routes), activeCount, version)
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
