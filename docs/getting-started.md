# Getting Started with NexusOps

This guide walks you through setting up NexusOps for your team.

## Prerequisites

- **Go 1.22+** — for building backend services
- **Docker & Docker Compose** — for running the platform
- **Node.js 20+** — for frontend development (optional)
- **PostgreSQL 16** — primary database
- **Redis 7** — caching and pub/sub

## Installation

### Option 1: Docker Compose (Recommended)

The fastest way to get NexusOps running:

```bash
git clone https://github.com/kasidit-wansudon/nexusops.git
cd nexusops

# Configure environment
cp .env.example .env
# Edit .env with your GitHub OAuth credentials and JWT secret

# Start all services
docker compose up -d
```

This starts:
- **API Server** on `http://localhost:8080`
- **Frontend Dashboard** on `http://localhost:3000`
- **Runner Agent** on `http://localhost:8081`
- **Reverse Proxy** on `http://localhost:80`
- **PostgreSQL** on `localhost:5432`
- **Redis** on `localhost:6379`
- **Prometheus** on `http://localhost:9090`
- **Grafana** on `http://localhost:3001` (admin/nexusops)

### Option 2: Build from Source

```bash
# Build all binaries
make build

# Run individual services
./bin/api --port 8080
./bin/runner --port 8081
./bin/proxy --port 80
```

### Option 3: Install CLI Only

```bash
go install github.com/kasidit-wansudon/nexusops/cmd/cli@latest
```

## Setting Up GitHub OAuth

1. Go to **GitHub Settings → Developer Settings → OAuth Apps → New OAuth App**
2. Set the callback URL to `http://localhost:8080/auth/github/callback`
3. Copy the Client ID and Client Secret to your `.env` file

## First Project

### 1. Initialize

```bash
cd your-project
nexusctl init my-service --template go
```

This creates a `nexusops.yaml` configuration file.

### 2. Configure Pipeline

Edit `nexusops.yaml` to define your build and deploy pipeline:

```yaml
name: my-service
version: "1.0"

pipeline:
  stages:
    - name: test
      steps:
        - name: unit-tests
          image: golang:1.22
          commands:
            - go test ./...

    - name: build
      steps:
        - name: build-binary
          image: golang:1.22
          commands:
            - CGO_ENABLED=0 go build -o /app/server ./cmd/api
          depends_on:
            - test

deploy:
  provider: docker
  strategy: rolling
  replicas: 2
  port: 8080
  health_check:
    path: /health
    interval: 30s
```

### 3. Deploy

```bash
nexusctl deploy --env production --strategy rolling
```

### 4. Monitor

```bash
# Check deployment status
nexusctl status --watch

# Stream logs
nexusctl logs --follow --level error

# View in dashboard
open http://localhost:3000
```

## Next Steps

- [Architecture Overview](architecture.md) — understand how NexusOps components work together
- [Pipeline Specification](pipeline-spec.md) — full YAML reference for pipelines
- [Deployment Strategies](deployment-strategies.md) — rolling, blue-green, and canary deployments
