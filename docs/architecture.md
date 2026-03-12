# Architecture

NexusOps is composed of four main services, each with distinct responsibilities.

## Service Overview

### API Server (`cmd/api`)

The central service handling all business logic, authentication, and data management.

**Responsibilities:**
- REST API for project, pipeline, deployment, and team management
- OAuth authentication (GitHub, GitLab) and API key validation
- WebSocket hub for real-time updates (build logs, deploy status, metrics)
- Webhook receiver for GitHub/GitLab push and PR events
- Environment variable encryption and management
- RBAC enforcement

**Key packages:**
- `internal/auth/` — OAuth providers, API key manager, session store
- `internal/project/` — Project config parsing, env var encryption, webhook handling
- `internal/pipeline/` — Pipeline YAML parser, artifact storage, build cache
- `internal/deploy/` — Docker/Kubernetes deployers, strategy engine, rollback manager
- `internal/monitor/` — Metrics collector, health checker, alert engine, log aggregator
- `internal/team/` — Member management, RBAC, activity feed

### Runner Agent (`cmd/runner`)

A worker service that executes CI/CD pipeline steps in isolated Docker containers.

**Responsibilities:**
- Accepts jobs from the API server via HTTP
- Parses pipeline YAML and validates step dependencies
- Executes steps in Docker containers with volume mounts and caching
- Streams build logs back to the API server in real-time
- Manages build artifacts with SHA256 checksums
- Handles concurrent job execution with configurable limits

**Architecture:**
```
Job Queue → Worker Pool (5 workers) → Docker API
                                          ↓
                                    Container Exec
                                          ↓
                                    Log Streaming → API Server
```

### Reverse Proxy (`cmd/proxy`)

An intelligent reverse proxy handling TLS termination, routing, and traffic management.

**Responsibilities:**
- Dynamic subdomain-to-container routing for preview environments
- Automatic TLS certificate management via Let's Encrypt (ACME)
- Load balancing with multiple algorithms (round-robin, weighted, least-connections)
- Token bucket rate limiting with per-IP tracking
- Admin API for route management

**Request flow:**
```
Client → TLS Termination → Rate Limiter → Router → Load Balancer → Backend
```

### CLI (`cmd/cli`)

The command-line interface for interacting with NexusOps.

**Commands:**
- `nexusctl init` — Initialize project with template
- `nexusctl deploy` — Trigger deployment
- `nexusctl logs` — View/stream logs
- `nexusctl env` — Manage environment variables
- `nexusctl status` — Check deployment status
- `nexusctl rollback` — Rollback deployment
- `nexusctl project` — Project CRUD operations
- `nexusctl pipeline` — Pipeline management

## Data Flow

### Deployment Flow

```
1. User runs `nexusctl deploy` or pushes to GitHub
2. API server receives request / webhook
3. Pipeline YAML is parsed and validated
4. Jobs are submitted to the Runner agent
5. Runner executes steps in Docker containers
6. Build artifacts are stored with checksums
7. Deployment strategy is executed (rolling/blue-green/canary)
8. Health checks verify the new deployment
9. Proxy routes are updated for the new containers
10. Notifications are sent (Slack/Discord/email)
11. Activity is logged in the audit feed
```

### Authentication Flow

```
1. User authenticates via GitHub/GitLab OAuth
2. Session is created with secure token
3. Subsequent requests include session cookie or API key
4. RBAC middleware checks permissions for each endpoint
5. API keys use "nxo_" prefix for easy identification
```

## Technology Choices

| Component | Choice | Rationale |
|-----------|--------|-----------|
| Language | Go | Performance, concurrency, strong typing, single binary deployment |
| Web Framework | Gin | Fast HTTP router, middleware support, production-proven |
| CLI Framework | Cobra | Industry standard for Go CLIs, auto-completion support |
| Frontend | Next.js 14 | App Router, SSR, TypeScript support, React ecosystem |
| Database | PostgreSQL | ACID compliance, JSON support, mature ecosystem |
| Cache | Redis | Pub/sub for real-time events, fast key-value cache |
| Container | Docker API | Direct API integration for container lifecycle management |
| Orchestration | Kubernetes client-go | Official Go client for K8s API |
| Metrics | Prometheus | Pull-based monitoring, widely adopted, rich ecosystem |
| TLS | ACME (Let's Encrypt) | Free, automated certificate management |

## Security

- **Encryption at rest**: Environment variables encrypted with AES-256-GCM
- **Password hashing**: bcrypt with cost factor 12
- **Webhook validation**: HMAC-SHA256 signature verification
- **API keys**: Cryptographically random with identifiable prefix
- **Sessions**: Secure, HTTP-only cookies with configurable TTL
- **RBAC**: Four-tier role system (admin, deployer, developer, viewer)
- **TLS**: Automatic certificate provisioning and renewal
