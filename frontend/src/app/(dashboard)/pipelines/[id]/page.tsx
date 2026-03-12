"use client";

import { useState } from "react";
import Link from "next/link";
import {
  ArrowLeft,
  GitBranch,
  GitCommit,
  Clock,
  User,
  RotateCcw,
  XCircle,
  Copy,
  Check,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { StageViewer, type PipelineStage } from "@/components/pipeline/stage-viewer";
import { BuildLog, type LogLine } from "@/components/pipeline/build-log";
import { formatDate, formatDuration, truncateHash } from "@/lib/utils";
import type { Status } from "@/lib/types";

function getStatusBadgeVariant(status: Status) {
  switch (status) {
    case "success":
      return "success" as const;
    case "running":
      return "info" as const;
    case "failed":
      return "error" as const;
    case "pending":
      return "warning" as const;
    default:
      return "secondary" as const;
  }
}

const pipeline = {
  id: "pipe-1",
  projectId: "proj-1",
  projectName: "nexus-api",
  status: "success" as Status,
  branch: "main",
  commit: "a1b2c3d4e5f6a7b8c9d0e1f2",
  commitMessage: "fix: resolve race condition in connection pool",
  triggeredBy: "John Doe",
  duration: 245,
  startedAt: new Date(Date.now() - 1000 * 60 * 20).toISOString(),
  finishedAt: new Date(Date.now() - 1000 * 60 * 16).toISOString(),
  createdAt: new Date(Date.now() - 1000 * 60 * 20).toISOString(),
};

const stages: PipelineStage[] = [
  {
    id: "stage-1",
    name: "Source",
    status: "success",
    steps: [
      {
        id: "step-1-1",
        name: "Checkout Code",
        status: "success",
        duration: 3,
        startedAt: pipeline.startedAt,
        finishedAt: new Date(
          new Date(pipeline.startedAt).getTime() + 3000
        ).toISOString(),
      },
      {
        id: "step-1-2",
        name: "Fetch Dependencies",
        status: "success",
        duration: 12,
        startedAt: pipeline.startedAt,
        finishedAt: new Date(
          new Date(pipeline.startedAt).getTime() + 15000
        ).toISOString(),
      },
    ],
  },
  {
    id: "stage-2",
    name: "Build",
    status: "success",
    steps: [
      {
        id: "step-2-1",
        name: "Compile",
        status: "success",
        duration: 45,
      },
      {
        id: "step-2-2",
        name: "Lint",
        status: "success",
        duration: 8,
      },
      {
        id: "step-2-3",
        name: "Build Docker Image",
        status: "success",
        duration: 36,
      },
    ],
  },
  {
    id: "stage-3",
    name: "Test",
    status: "success",
    steps: [
      {
        id: "step-3-1",
        name: "Unit Tests",
        status: "success",
        duration: 67,
      },
      {
        id: "step-3-2",
        name: "Integration Tests",
        status: "success",
        duration: 43,
      },
      {
        id: "step-3-3",
        name: "Coverage Report",
        status: "success",
        duration: 10,
      },
    ],
  },
  {
    id: "stage-4",
    name: "Security",
    status: "success",
    steps: [
      {
        id: "step-4-1",
        name: "SAST Scan",
        status: "success",
        duration: 22,
      },
      {
        id: "step-4-2",
        name: "Dependency Check",
        status: "success",
        duration: 15,
      },
    ],
  },
  {
    id: "stage-5",
    name: "Deploy",
    status: "success",
    steps: [
      {
        id: "step-5-1",
        name: "Push to Registry",
        status: "success",
        duration: 18,
      },
      {
        id: "step-5-2",
        name: "Deploy to Staging",
        status: "success",
        duration: 25,
      },
      {
        id: "step-5-3",
        name: "Health Check",
        status: "success",
        duration: 8,
      },
    ],
  },
];

const mockLogLines: LogLine[] = [
  { lineNumber: 1, content: "\x1b[36m[info]\x1b[0m Starting build process...", timestamp: "12:04:15" },
  { lineNumber: 2, content: "\x1b[36m[info]\x1b[0m Pulling base image: golang:1.22-alpine", timestamp: "12:04:16" },
  { lineNumber: 3, content: "\x1b[36m[info]\x1b[0m Step 1/12: FROM golang:1.22-alpine AS builder", timestamp: "12:04:17" },
  { lineNumber: 4, content: "\x1b[36m[info]\x1b[0m Step 2/12: WORKDIR /app", timestamp: "12:04:17" },
  { lineNumber: 5, content: "\x1b[36m[info]\x1b[0m Step 3/12: COPY go.mod go.sum ./", timestamp: "12:04:18" },
  { lineNumber: 6, content: "\x1b[36m[info]\x1b[0m Step 4/12: RUN go mod download", timestamp: "12:04:18" },
  { lineNumber: 7, content: "  go: downloading github.com/gofiber/fiber/v2 v2.52.0", timestamp: "12:04:20" },
  { lineNumber: 8, content: "  go: downloading github.com/jackc/pgx/v5 v5.5.3", timestamp: "12:04:21" },
  { lineNumber: 9, content: "  go: downloading github.com/redis/go-redis/v9 v9.4.0", timestamp: "12:04:22" },
  { lineNumber: 10, content: "  go: downloading go.uber.org/zap v1.26.0", timestamp: "12:04:23" },
  { lineNumber: 11, content: "\x1b[36m[info]\x1b[0m Step 5/12: COPY . .", timestamp: "12:04:25" },
  { lineNumber: 12, content: "\x1b[36m[info]\x1b[0m Step 6/12: RUN CGO_ENABLED=0 go build -o /app/server ./cmd/server", timestamp: "12:04:26" },
  { lineNumber: 13, content: "  Building nexus-api server...", timestamp: "12:04:30" },
  { lineNumber: 14, content: "\x1b[32m[success]\x1b[0m Binary compiled successfully (14.2 MB)", timestamp: "12:04:55" },
  { lineNumber: 15, content: "\x1b[36m[info]\x1b[0m Step 7/12: FROM alpine:3.19", timestamp: "12:04:56" },
  { lineNumber: 16, content: "\x1b[36m[info]\x1b[0m Step 8/12: RUN apk add --no-cache ca-certificates", timestamp: "12:04:57" },
  { lineNumber: 17, content: "\x1b[36m[info]\x1b[0m Step 9/12: COPY --from=builder /app/server /server", timestamp: "12:04:58" },
  { lineNumber: 18, content: "\x1b[36m[info]\x1b[0m Step 10/12: EXPOSE 8080", timestamp: "12:04:58" },
  { lineNumber: 19, content: "\x1b[36m[info]\x1b[0m Step 11/12: HEALTHCHECK CMD wget -q --spider http://localhost:8080/health", timestamp: "12:04:59" },
  { lineNumber: 20, content: "\x1b[36m[info]\x1b[0m Step 12/12: CMD [\"/server\"]", timestamp: "12:04:59" },
  { lineNumber: 21, content: "\x1b[32m[success]\x1b[0m Docker image built: nexusops/nexus-api:a1b2c3d", timestamp: "12:05:00" },
  { lineNumber: 22, content: "\x1b[32m[success]\x1b[0m Image size: 28.4 MB", timestamp: "12:05:00" },
  { lineNumber: 23, content: "", timestamp: "" },
  { lineNumber: 24, content: "\x1b[1m\x1b[36m=== Running Unit Tests ===\x1b[0m", timestamp: "12:05:01" },
  { lineNumber: 25, content: "  ok   nexus-api/internal/auth       0.234s  coverage: 94.2%", timestamp: "12:05:10" },
  { lineNumber: 26, content: "  ok   nexus-api/internal/database    0.567s  coverage: 88.7%", timestamp: "12:05:15" },
  { lineNumber: 27, content: "  ok   nexus-api/internal/handler     0.892s  coverage: 91.3%", timestamp: "12:05:22" },
  { lineNumber: 28, content: "  ok   nexus-api/internal/middleware  0.123s  coverage: 96.1%", timestamp: "12:05:25" },
  { lineNumber: 29, content: "  ok   nexus-api/internal/service     1.245s  coverage: 87.4%", timestamp: "12:05:35" },
  { lineNumber: 30, content: "\x1b[32m[success]\x1b[0m All 127 tests passed. Overall coverage: 91.5%", timestamp: "12:05:35" },
  { lineNumber: 31, content: "", timestamp: "" },
  { lineNumber: 32, content: "\x1b[1m\x1b[36m=== Security Scan ===\x1b[0m", timestamp: "12:05:36" },
  { lineNumber: 33, content: "  Scanning for vulnerabilities...", timestamp: "12:05:40" },
  { lineNumber: 34, content: "  \x1b[33m[warn]\x1b[0m Low severity: CVE-2024-1234 in indirect dependency (informational only)", timestamp: "12:05:50" },
  { lineNumber: 35, content: "\x1b[32m[success]\x1b[0m No critical or high vulnerabilities found", timestamp: "12:05:58" },
  { lineNumber: 36, content: "", timestamp: "" },
  { lineNumber: 37, content: "\x1b[1m\x1b[36m=== Deploying to Staging ===\x1b[0m", timestamp: "12:06:00" },
  { lineNumber: 38, content: "  Pushing image to registry...", timestamp: "12:06:02" },
  { lineNumber: 39, content: "\x1b[32m[success]\x1b[0m Image pushed: registry.nexusops.dev/nexus-api:a1b2c3d", timestamp: "12:06:18" },
  { lineNumber: 40, content: "  Updating Kubernetes deployment...", timestamp: "12:06:20" },
  { lineNumber: 41, content: "  Waiting for rollout to complete...", timestamp: "12:06:25" },
  { lineNumber: 42, content: "  deployment.apps/nexus-api successfully rolled out", timestamp: "12:06:43" },
  { lineNumber: 43, content: "  Running health check on https://staging-api.nexusops.dev/health", timestamp: "12:06:44" },
  { lineNumber: 44, content: "\x1b[32m[success]\x1b[0m Health check passed (response time: 23ms)", timestamp: "12:06:51" },
  { lineNumber: 45, content: "", timestamp: "" },
  { lineNumber: 46, content: "\x1b[1m\x1b[32m=== Pipeline Complete ===\x1b[0m", timestamp: "12:06:51" },
  { lineNumber: 47, content: "\x1b[32m[success]\x1b[0m All stages passed in 4m 5s", timestamp: "12:06:51" },
];

export default function PipelineDetailPage() {
  const [activeStepId, setActiveStepId] = useState<string>("step-2-3");
  const [copied, setCopied] = useState(false);

  const handleStepClick = (_stageId: string, stepId: string) => {
    setActiveStepId(stepId);
  };

  const copyCommitHash = () => {
    navigator.clipboard.writeText(pipeline.commit);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <Link
          href="/pipelines"
          className="mb-4 inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground transition-colors"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          Back to Pipelines
        </Link>

        <div className="flex items-start justify-between">
          <div>
            <div className="flex items-center gap-3">
              <h1 className="text-xl font-bold tracking-tight">
                Pipeline #{pipeline.id.split("-")[1]}
              </h1>
              <Badge variant={getStatusBadgeVariant(pipeline.status)} dot>
                {pipeline.status}
              </Badge>
            </div>
            <p className="mt-1 text-sm text-muted-foreground">
              {pipeline.commitMessage}
            </p>
            <div className="mt-3 flex flex-wrap items-center gap-4 text-xs text-muted-foreground">
              <span className="flex items-center gap-1">
                <GitBranch className="h-3 w-3" />
                {pipeline.branch}
              </span>
              <button
                onClick={copyCommitHash}
                className="flex items-center gap-1 font-mono hover:text-foreground transition-colors"
              >
                <GitCommit className="h-3 w-3" />
                {truncateHash(pipeline.commit)}
                {copied ? (
                  <Check className="h-3 w-3 text-nexus-green-400" />
                ) : (
                  <Copy className="h-3 w-3" />
                )}
              </button>
              <span className="flex items-center gap-1">
                <User className="h-3 w-3" />
                {pipeline.triggeredBy}
              </span>
              <span className="flex items-center gap-1">
                <Clock className="h-3 w-3" />
                {formatDuration(pipeline.duration)}
              </span>
              <span>{formatDate(pipeline.startedAt)}</span>
            </div>
          </div>
          <div className="flex items-center gap-2">
            {pipeline.status === "failed" && (
              <Button size="sm" variant="outline">
                <RotateCcw className="mr-2 h-3.5 w-3.5" />
                Retry
              </Button>
            )}
            {pipeline.status === "running" && (
              <Button size="sm" variant="destructive">
                <XCircle className="mr-2 h-3.5 w-3.5" />
                Cancel
              </Button>
            )}
          </div>
        </div>
      </div>

      {/* Stage Visualization */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Stages</CardTitle>
        </CardHeader>
        <CardContent>
          <StageViewer
            stages={stages}
            activeStepId={activeStepId}
            onStepClick={handleStepClick}
          />
        </CardContent>
      </Card>

      {/* Build Log */}
      <div>
        <h2 className="text-base font-semibold mb-3">Build Output</h2>
        <BuildLog
          lines={mockLogLines}
          title={`Build Log - ${stages.find((s) =>
            s.steps.find((st) => st.id === activeStepId)
          )?.name || "Pipeline"} / ${stages
            .flatMap((s) => s.steps)
            .find((st) => st.id === activeStepId)?.name || "Output"}`}
          streaming={pipeline.status === "running"}
          maxHeight="500px"
        />
      </div>
    </div>
  );
}
