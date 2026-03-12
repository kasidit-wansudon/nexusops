# NexusOps

**Self-hosted developer platform combining CI/CD, deployment, monitoring, and team collaboration.**

NexusOps is a complete platform for teams who want full control over their development infrastructure without vendor lock-in. Deploy to Docker or Kubernetes with built-in pipeline execution, smart reverse proxy, real-time monitoring, and team management.

[![CI](https://github.com/kasidit-wansudon/nexusops/actions/workflows/ci.yml/badge.svg)](https://github.com/kasidit-wansudon/nexusops/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/kasidit-wansudon/nexusops)](https://goreportcard.com/report/github.com/kasidit-wansudon/nexusops)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        NexusOps Platform                        │
├──────────┬──────────┬──────────┬──────────┬─────────────────────┤
│  CLI     │ Frontend │  Proxy   │   API    │     Runner          │
│ nexusctl │ Next.js  │ Reverse  │  Server  │   Pipeline Agent    │
│          │ Dashboard│ Proxy    │          │                     │
├──────────┴──────────┴────┬─────┴──────────┴─────────────────────┤
│                          │                                      │
│  ┌─────────────┐  ┌──────┴──────┐  ┌──────────────────────┐    │
│  │   Auth      │  │  Pipeline   │  │    Deploy Engine      │    │
│  │  OAuth/API  │  │   Engine    │  │  Docker / Kubernetes  │    │
│  │  Sessions   │  │  Parser     │  │  Rolling / Blue-Green │    │
│  └─────────────┘  │  Runner     │  │  Canary / Preview     │    │
│                    │  Artifacts  │  └──────────────────────┘    │
│  ┌─────────────┐  │  Cache      │                               │
│  │  Project    │  └─────────────┘  ┌──────────────────────┐    │
│  │  Config     │                    │    Monitoring         │    │
│  │  Env Vars   │  ┌─────────────┐  │  Metrics / Health    │    │
│  │  Webhooks   │  │   Team      │  │  Alerts / Logs       │    │
│  └─────────────┘  │  RBAC       │  └──────────────────────┘    │
│                    │  Activity   │                               │
│                    └─────────────┘                               │
├─────────────────────────────────────────────────────────────────┤
│              PostgreSQL  │  Redis  │  Prometheus  │  Grafana    │
└─────────────────────────────────────────────────────────────────┘
```

## Features

| Category | Feature | Description |
|----------|---------|-------------|
| **CI/CD** | Pipeline Engine | YAML-defined pipelines with DAG dependency resolution |
| | Container Execution | Isolated container-based step execution via Docker API |
| | Build Cache | LRU-based layer caching for faster rebuilds |
| | Artifacts | SHA256-checksummed artifact storage and retrieval |
| **Deploy** | Multi-Target | Deploy to Docker Compose or Kubernetes |
| | Strategies | Rolling update, blue-green, and canary deployments |
| | Preview Envs | Automatic preview environments per pull request |
| | Rollback | One-click rollback with automatic health check validation |
| **Proxy** | Smart Routing | Dynamic subdomain-to-container routing |
| | Load Balancing | Round-robin, weighted, and least-connections algorithms |
| | Auto TLS | Automatic TLS certificates via Let's Encrypt (ACME) |
| | Rate Limiting | Token bucket rate limiter with per-key tracking |
| **Monitor** | Metrics | Prometheus-compatible metric collection and export |
| | Health Checks | HTTP, TCP, and command-based health check engine |
| | Alerting | Rules engine with Slack, Discord, email, and webhook channels |
| | Log Aggregation | Centralized log collection with Loki-compatible storage |
| **Team** | RBAC | Role-based access control (admin, deployer, developer, viewer) |
| | Activity Feed | Complete audit log of all platform actions |
| | Notifications | Multi-channel dispatcher (Slack, Discord, email, webhook) |
| **Auth** | OAuth | GitHub and GitLab OAuth provider integration |
| | API Keys | Prefixed API key generation and validation (`nxo_*`) |
| | Sessions | Secure session management with configurable TTL |

## Quick Start

### Prerequisites

- Go 1.22+
- Docker & Docker Compose
- Node.js 20+ (for frontend development)

### Development Setup

```bash
# Clone the repository
git clone https://github.com/kasidit-wansudon/nexusops.git
cd nexusops

# Start all services
docker compose up -d

# Or run individual components
make run-api      # API server on :8080
make run-runner   # Runner agent on :8081
make run-proxy    # Reverse proxy on :80

# Frontend development
make frontend-dev # Next.js dev server on :3000
```

### Using the CLI

```bash
# Install the CLI
go install ./cmd/cli@latest

# Initialize a project
nexusctl init my-project --template go

# Deploy
nexusctl deploy --env production --strategy rolling

# Stream logs
nexusctl logs --follow --level error

# Check status
nexusctl status --watch

# Manage environment variables
nexusctl env set DATABASE_URL=postgres://...
nexusctl env list

# Pipeline operations
nexusctl pipeline trigger --branch main
nexusctl pipeline status
```

### Pipeline Configuration

Create a `nexusops.yaml` in your project root:

```yaml
name: my-service
version: "1.0"

build:
  dockerfile: Dockerfile
  context: .
  cache:
    - node_modules
    - .go/pkg

pipeline:
  stages:
    - name: test
      steps:
        - name: lint
          image: golangci/golangci-lint:latest
          commands:
            - golangci-lint run ./...
        - name: unit-tests
          image: golang:1.22
          commands:
            - go test -race ./...
          cache:
            key: go-mod-${CHECKSUM:go.sum}
            paths:
              - /go/pkg/mod

    - name: build
      steps:
        - name: build-binary
          image: golang:1.22
          commands:
            - CGO_ENABLED=0 go build -o /app/server ./cmd/api
          artifacts:
            paths:
              - /app/server
          depends_on:
            - test

    - name: deploy
      steps:
        - name: deploy-production
          image: nexusops/deployer:latest
          commands:
            - nexusctl deploy --image ${IMAGE} --env production
          depends_on:
            - build

deploy:
  provider: docker
  strategy: rolling
  replicas: 2
  port: 8080
  health_check:
    path: /health
    interval: 30s
    timeout: 10s
    retries: 3
```

## Project Structure

```
nexusops/
├── cmd/
│   ├── api/          # API server entry point
│   ├── runner/       # Pipeline runner agent
│   ├── proxy/        # Reverse proxy server
│   └── cli/          # CLI entry point
├── cli/
│   ├── cmd/          # CLI commands (cobra)
│   └── config/       # CLI configuration
├── internal/
│   ├── auth/         # Authentication (OAuth, API keys, sessions)
│   ├── deploy/       # Deployment engine (Docker, K8s, strategies)
│   ├── monitor/      # Monitoring (metrics, health, alerts, logs)
│   ├── notification/ # Notification dispatcher
│   ├── pipeline/     # CI/CD pipeline (parser, runner, artifacts, cache)
│   ├── pkg/          # Shared packages (config, crypto, Docker, git, K8s, WS)
│   ├── project/      # Project management (config, env, webhooks)
│   ├── proxy/        # Reverse proxy (router, TLS, LB, rate limit)
│   └── team/         # Team management (members, RBAC, activity)
├── frontend/         # Next.js 14 dashboard
├── deploy/
│   ├── docker/       # Dockerfiles (multi-stage builds)
│   ├── k8s/          # Kubernetes manifests
│   ├── terraform/    # Terraform modules (AWS)
│   └── prometheus/   # Prometheus configuration
├── examples/         # Example projects (Go, Node.js, Laravel)
└── docs/             # Documentation
```

## Deployment

### Docker Compose (Recommended for small teams)

```bash
# Configure environment
cp .env.example .env
# Edit .env with your settings

# Deploy
docker compose up -d

# View logs
docker compose logs -f api
```

### Kubernetes

```bash
# Apply manifests
kubectl apply -f deploy/k8s/namespace.yaml
kubectl apply -f deploy/k8s/

# Check status
kubectl get pods -n nexusops
```

### Terraform (AWS)

```bash
cd deploy/terraform
terraform init
terraform plan -var="environment=production"
terraform apply
```

## API Reference

### Authentication

All API requests require authentication via Bearer token or API key:

```bash
# Using API key
curl -H "Authorization: Bearer nxo_your_api_key" \
  http://localhost:8080/api/v1/projects

# OAuth flow
# 1. Redirect to /auth/github or /auth/gitlab
# 2. Handle callback with session token
```

### Core Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Health check |
| `GET` | `/ready` | Readiness check |
| `GET` | `/metrics` | Prometheus metrics |
| `GET` | `/api/v1/projects` | List projects |
| `POST` | `/api/v1/projects` | Create project |
| `GET` | `/api/v1/projects/:id` | Get project |
| `POST` | `/api/v1/projects/:id/deploy` | Trigger deployment |
| `POST` | `/api/v1/projects/:id/pipelines/trigger` | Trigger pipeline |
| `GET` | `/api/v1/projects/:id/logs` | Fetch logs |
| `GET` | `/api/v1/projects/:id/logs/stream` | Stream logs (SSE) |
| `GET` | `/api/v1/projects/:id/env` | List env vars |
| `PUT` | `/api/v1/projects/:id/env` | Set env vars |
| `POST` | `/api/v1/webhooks/github` | GitHub webhook receiver |
| `GET` | `/api/v1/teams` | List teams |
| `GET` | `/api/v1/activity` | Activity feed |
| `WS` | `/ws` | WebSocket (real-time updates) |

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_HOST` | `localhost` | PostgreSQL host |
| `DATABASE_PORT` | `5432` | PostgreSQL port |
| `DATABASE_NAME` | `nexusops` | Database name |
| `DATABASE_USER` | `nexusops` | Database user |
| `DATABASE_PASSWORD` | - | Database password |
| `REDIS_HOST` | `localhost` | Redis host |
| `REDIS_PORT` | `6379` | Redis port |
| `GITHUB_CLIENT_ID` | - | GitHub OAuth app client ID |
| `GITHUB_CLIENT_SECRET` | - | GitHub OAuth app client secret |
| `JWT_SECRET` | - | JWT signing secret |
| `TLS_EMAIL` | - | Email for Let's Encrypt |

## Development

```bash
# Run tests
make test

# Run tests with coverage
make test-coverage

# Lint
make lint

# Format code
make fmt

# Build all binaries
make build

# Build Docker images
make docker-build
```

## Tech Stack

- **Backend**: Go 1.22, Gin, Docker API, Kubernetes client-go
- **Frontend**: Next.js 14, TypeScript, Tailwind CSS
- **CLI**: Cobra, tabwriter
- **Database**: PostgreSQL 16, Redis 7
- **Monitoring**: Prometheus, Grafana
- **Infrastructure**: Docker, Kubernetes, Terraform (AWS)

## License

MIT License — see [LICENSE](LICENSE) for details.

---

Built by [Kasidit Wansudon](mailto:kasidit.wans@gmail.com)
