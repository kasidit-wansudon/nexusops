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

	"github.com/gin-gonic/gin"
	"github.com/kasidit-wansudon/nexusops/internal/auth/apikey"
	"github.com/kasidit-wansudon/nexusops/internal/auth/oauth"
	"github.com/kasidit-wansudon/nexusops/internal/auth/session"
	"github.com/kasidit-wansudon/nexusops/internal/monitor/health"
	"github.com/kasidit-wansudon/nexusops/internal/monitor/metrics"
	"github.com/kasidit-wansudon/nexusops/internal/notification"
	"github.com/kasidit-wansudon/nexusops/internal/pipeline/artifact"
	"github.com/kasidit-wansudon/nexusops/internal/pipeline/cache"
	plog "github.com/kasidit-wansudon/nexusops/internal/pipeline/log"
	"github.com/kasidit-wansudon/nexusops/internal/pipeline/parser"
	"github.com/kasidit-wansudon/nexusops/internal/pipeline/runner"
	"github.com/kasidit-wansudon/nexusops/internal/pkg/config"
	"github.com/docker/docker/client"
	"github.com/kasidit-wansudon/nexusops/internal/pkg/crypto"
	pkgdocker "github.com/kasidit-wansudon/nexusops/internal/pkg/docker"
	"github.com/kasidit-wansudon/nexusops/internal/pkg/ws"
	"github.com/kasidit-wansudon/nexusops/internal/project/env"
	projectgit "github.com/kasidit-wansudon/nexusops/internal/project/git"
	"github.com/kasidit-wansudon/nexusops/internal/team/activity"
	"github.com/kasidit-wansudon/nexusops/internal/team/member"
	"github.com/kasidit-wansudon/nexusops/internal/team/role"
)

const (
	version   = "1.0.0"
	banner    = "NexusOps API Server v%s\n"
	readTimeout  = 15 * time.Second
	writeTimeout = 15 * time.Second
	idleTimeout  = 60 * time.Second
)

func main() {
	fmt.Printf(banner, version)

	cfg := config.Load()

	encryptionKey, err := crypto.GenerateKey()
	if err != nil {
		log.Fatalf("Failed to generate encryption key: %v", err)
	}

	sessionStore := session.NewMemoryStore(24 * time.Hour)
	apiKeyManager := apikey.NewManager(encryptionKey)
	metricsCollector := metrics.NewCollector()
	healthChecker := health.NewChecker(30 * time.Second)
	wsHub := ws.NewHub()
	activityFeed := activity.NewFeed(10000)
	teamManager := member.NewManager()
	rbac := role.NewRBAC()
	envManager, err := env.NewManager(encryptionKey)
	if err != nil {
		log.Fatalf("Failed to create env manager: %v", err)
	}
	logStreamer := plog.NewStreamer(10000)
	artifactStore, err := artifact.NewStore("/tmp/nexusops/artifacts")
	if err != nil {
		log.Fatalf("Failed to create artifact store: %v", err)
	}
	buildCache, err := cache.NewCache("/tmp/nexusops/cache", 1<<30)
	if err != nil {
		log.Fatalf("Failed to create build cache: %v", err)
	}
	dispatcher := notification.NewDispatcher(4)

	_ = parser.ParsePipeline
	_ = buildCache

	dockerClient, err := pkgdocker.NewDockerClient()
	if err != nil {
		log.Printf("Warning: Docker client not available: %v", err)
	}

	var apiClient client.APIClient
	if dockerClient != nil {
		apiClient = dockerClient.APIClient()
	}
	pipelineRunner := runner.NewRunner(apiClient, logStreamer, artifactStore)
	_ = pipelineRunner

	githubOAuth := oauth.NewGitHubProvider(oauth.Config{
		ClientID:     cfg.Auth.GitHubClientID,
		ClientSecret: cfg.Auth.GitHubSecret,
		RedirectURL:  fmt.Sprintf("http://%s:%d/auth/github/callback", cfg.Server.Host, cfg.Server.Port),
		Scopes:       []string{"user:email", "repo"},
	})
	_ = githubOAuth

	gitlabOAuth := oauth.NewGitLabProvider(oauth.Config{
		ClientID:     cfg.Auth.GitLabClientID,
		ClientSecret: cfg.Auth.GitLabSecret,
		RedirectURL:  fmt.Sprintf("http://%s:%d/auth/gitlab/callback", cfg.Server.Host, cfg.Server.Port),
		Scopes:       []string{"read_user", "api"},
	})
	_ = gitlabOAuth

	healthChecker.Register("api", &health.HTTPCheck{
		URL:            fmt.Sprintf("http://localhost:%d/health", cfg.Server.Port),
		Method:         "GET",
		ExpectedStatus: 200,
		Timeout:        5 * time.Second,
	})

	go wsHub.Run()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	healthChecker.Start(ctx)
	dispatcher.Start(ctx)

	router := setupRouter(cfg, sessionStore, apiKeyManager, metricsCollector,
		healthChecker, wsHub, activityFeed, teamManager, rbac, envManager,
		logStreamer, dispatcher, githubOAuth, gitlabOAuth, dockerClient, pipelineRunner)

	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	go func() {
		log.Printf("API server listening on %s:%d", cfg.Server.Host, cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced shutdown: %v", err)
	}

	healthChecker.Stop()
	sessionStore.Cleanup()

	log.Println("Server exited gracefully")
}

func setupRouter(
	cfg *config.AppConfig,
	sessionStore *session.MemoryStore,
	apiKeyManager *apikey.Manager,
	metricsCollector *metrics.Collector,
	healthChecker *health.Checker,
	wsHub *ws.Hub,
	activityFeed *activity.Feed,
	teamManager *member.Manager,
	rbac *role.RBAC,
	envManager *env.Manager,
	logStreamer *plog.Streamer,
	dispatcher *notification.Dispatcher,
	githubOAuth *oauth.GitHubProvider,
	gitlabOAuth *oauth.GitLabProvider,
	dockerClient *pkgdocker.DockerClient,
	pipelineRunner *runner.Runner,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(requestLogger(metricsCollector))
	r.Use(corsMiddleware())

	r.GET("/health", gin.WrapH(healthChecker.Handler()))
	r.GET("/ready", gin.WrapH(healthChecker.ReadinessHandler()))
	r.GET("/metrics", gin.WrapH(metricsCollector.Handler()))

	auth := r.Group("/auth")
	{
		auth.GET("/github", func(c *gin.Context) {
			state, _ := crypto.GenerateRandomToken(32)
			c.Redirect(http.StatusTemporaryRedirect, githubOAuth.AuthURL(state))
		})
		auth.GET("/github/callback", func(c *gin.Context) {
			code := c.Query("code")
			if code == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "missing code parameter"})
				return
			}
			token, err := githubOAuth.Exchange(c.Request.Context(), code)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "oauth exchange failed"})
				return
			}
			user, err := githubOAuth.GetUser(c.Request.Context(), token.AccessToken)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user"})
				return
			}
			sess, err := sessionStore.Create(fmt.Sprintf("%d", user.ID), map[string]string{
				"login":    user.Login,
				"provider": "github",
			})
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "session creation failed"})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"token": sess.Token,
				"user":  user,
			})
		})
		auth.GET("/gitlab", func(c *gin.Context) {
			state, _ := crypto.GenerateRandomToken(32)
			c.Redirect(http.StatusTemporaryRedirect, gitlabOAuth.AuthURL(state))
		})
		auth.GET("/gitlab/callback", func(c *gin.Context) {
			code := c.Query("code")
			if code == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "missing code parameter"})
				return
			}
			token, err := gitlabOAuth.Exchange(c.Request.Context(), code)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "oauth exchange failed"})
				return
			}
			user, err := gitlabOAuth.GetUser(c.Request.Context(), token.AccessToken)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user"})
				return
			}
			sess, err := sessionStore.Create(fmt.Sprintf("%d", user.ID), map[string]string{
				"login":    user.Login,
				"provider": "gitlab",
			})
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "session creation failed"})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"token": sess.Token,
				"user":  user,
			})
		})
		auth.POST("/api-keys", session.AuthMiddleware(sessionStore), func(c *gin.Context) {
			var req struct {
				Name        string   `json:"name" binding:"required"`
				ProjectID   string   `json:"project_id"`
				Permissions []string `json:"permissions"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			userID := c.GetString("user_id")
			key, err := apiKeyManager.Generate(req.Name, req.ProjectID, userID, req.Permissions)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusCreated, key)
		})
		auth.GET("/api-keys", session.AuthMiddleware(sessionStore), func(c *gin.Context) {
			userID := c.GetString("user_id")
			keys := apiKeyManager.List(userID)
			c.JSON(http.StatusOK, keys)
		})
	}

	api := r.Group("/api/v1")
	api.Use(session.AuthMiddleware(sessionStore))
	{
		api.POST("/projects", func(c *gin.Context) {
			var req struct {
				Name        string `json:"name" binding:"required"`
				Repository  string `json:"repository" binding:"required"`
				Description string `json:"description"`
				TeamID      string `json:"team_id"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			activityFeed.Record(&activity.Event{
				Type:         "project.created",
				ActorID:      c.GetString("user_id"),
				Description:  fmt.Sprintf("Created project %s", req.Name),
				ResourceType: "project",
			})
			c.JSON(http.StatusCreated, gin.H{"name": req.Name, "repository": req.Repository})
		})

		api.POST("/projects/:id/env", func(c *gin.Context) {
			projectID := c.Param("id")
			var req struct {
				Key         string `json:"key" binding:"required"`
				Value       string `json:"value" binding:"required"`
				Environment string `json:"environment"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if req.Environment == "" {
				req.Environment = "production"
			}
			if err := envManager.Set(projectID, req.Key, req.Value, req.Environment); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		api.GET("/projects/:id/env", func(c *gin.Context) {
			projectID := c.Param("id")
			environment := c.DefaultQuery("environment", "production")
			vars, err := envManager.List(projectID, environment)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, vars)
		})

		api.POST("/projects/:id/deploy", func(c *gin.Context) {
			var req struct {
				Environment string `json:"environment"`
				Image       string `json:"image" binding:"required"`
				Strategy    string `json:"strategy"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			activityFeed.Record(&activity.Event{
				Type:         "deploy.started",
				ActorID:      c.GetString("user_id"),
				ProjectID:    c.Param("id"),
				Description:  fmt.Sprintf("Deployment started for %s", req.Image),
				ResourceType: "deployment",
			})
			c.JSON(http.StatusAccepted, gin.H{"status": "deploying", "image": req.Image})
		})

		api.POST("/projects/:id/pipelines/trigger", func(c *gin.Context) {
			projectID := c.Param("id")
			var req struct {
				Branch string `json:"branch"`
				Config string `json:"config"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			activityFeed.Record(&activity.Event{
				Type:         "pipeline.triggered",
				ActorID:      c.GetString("user_id"),
				ProjectID:    projectID,
				Description:  fmt.Sprintf("Pipeline triggered on branch %s", req.Branch),
				ResourceType: "pipeline",
			})
			c.JSON(http.StatusAccepted, gin.H{"status": "triggered", "project_id": projectID})
		})

		api.POST("/webhooks/github", func(c *gin.Context) {
			event, err := projectgit.ParseGitHubWebhook(c.Request, cfg.Auth.GitHubSecret)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			activityFeed.Record(&activity.Event{
				Type:        "webhook.received",
				Description: fmt.Sprintf("GitHub %s event on %s", event.Type, event.Branch),
			})
			c.JSON(http.StatusOK, gin.H{"status": "processed", "event": event.Type})
		})

		api.POST("/teams", func(c *gin.Context) {
			var req struct {
				Name        string `json:"name" binding:"required"`
				Description string `json:"description"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			team, err := teamManager.CreateTeam(req.Name, req.Description, c.GetString("user_id"))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusCreated, team)
		})

		api.POST("/teams/:id/members", func(c *gin.Context) {
			teamID := c.Param("id")
			var req struct {
				UserID string `json:"user_id" binding:"required"`
				Role   string `json:"role"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if req.Role == "" {
				req.Role = "developer"
			}
			m, err := teamManager.AddMember(teamID, req.UserID, req.Role, c.GetString("user_id"))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusCreated, m)
		})

		api.GET("/teams/:id/members", func(c *gin.Context) {
			members, err := teamManager.ListMembers(c.Param("id"))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, members)
		})

		api.GET("/activity/:project_id", func(c *gin.Context) {
			events := activityFeed.GetByProject(c.Param("project_id"), 50)
			c.JSON(http.StatusOK, events)
		})

		api.POST("/notifications/test", func(c *gin.Context) {
			n := notification.BuildDeployNotification("test-project", "production", "success", "v1.0.0")
			if err := dispatcher.Send(c.Request.Context(), n); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "sent"})
		})
	}

	r.GET("/ws", func(c *gin.Context) {
		wsHub.HandleWebSocket(c.Writer, c.Request)
	})

	api.POST("/roles/assign", func(c *gin.Context) {
		var req struct {
			UserID    string `json:"user_id" binding:"required"`
			ProjectID string `json:"project_id" binding:"required"`
			Role      string `json:"role" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		rbac.AssignRole(req.UserID, req.ProjectID, req.Role)
		c.JSON(http.StatusOK, gin.H{"status": "assigned"})
	})

	return r
}

func requestLogger(collector *metrics.Collector) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)
		collector.RecordHTTPRequest(
			c.Request.Method,
			c.FullPath(),
			fmt.Sprintf("%d", c.Writer.Status()),
			duration.Seconds(),
		)
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-API-Key")
		c.Header("Access-Control-Max-Age", "86400")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
