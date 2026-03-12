"use client";

import { useState } from "react";
import Link from "next/link";
import {
  GitBranch,
  Clock,
  User,
  RefreshCw,
  XCircle,
  CheckCircle2,
  Loader2,
  Filter,
  Search,
  Play,
  RotateCcw,
} from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { PipelineStatus } from "@/components/pipeline-status";
import { cn, formatDate, formatDuration, truncateHash } from "@/lib/utils";
import type { Status, PipelineRun, PipelineStep } from "@/lib/types";

const mockUser = {
  id: "u1",
  name: "John Doe",
  email: "john@nexusops.dev",
  avatarUrl: "",
  role: "admin" as const,
  lastActive: new Date().toISOString(),
  createdAt: "2024-01-01T00:00:00Z",
};

const mockUsers = [
  mockUser,
  { ...mockUser, id: "u2", name: "Sarah Chen", email: "sarah@nexusops.dev", role: "developer" as const },
  { ...mockUser, id: "u3", name: "Mike Wilson", email: "mike@nexusops.dev", role: "developer" as const },
  { ...mockUser, id: "u4", name: "Alex Rivera", email: "alex@nexusops.dev", role: "developer" as const },
];

function makeSteps(statuses: Array<PipelineStep["status"]>): PipelineStep[] {
  const names = ["Checkout", "Install", "Lint", "Test", "Build", "Deploy"];
  return names.map((name, i) => ({
    id: `s${i}`,
    name,
    status: statuses[i] || "pending",
    duration: statuses[i] === "success" ? Math.floor(Math.random() * 120) + 10 : 0,
    startedAt: "",
    finishedAt: null,
  }));
}

const mockPipelines: PipelineRun[] = [
  {
    id: "pipe-1",
    projectId: "proj-1",
    projectName: "nexus-api",
    status: "success",
    branch: "main",
    commit: "a1b2c3d4e5f6a7b8",
    commitMessage: "Fix auth token refresh logic",
    triggeredBy: mockUsers[0],
    steps: makeSteps(["success", "success", "success", "success", "success", "success"]),
    duration: 245,
    startedAt: new Date(Date.now() - 1000 * 60 * 20).toISOString(),
    finishedAt: new Date(Date.now() - 1000 * 60 * 16).toISOString(),
    createdAt: new Date(Date.now() - 1000 * 60 * 20).toISOString(),
  },
  {
    id: "pipe-2",
    projectId: "proj-2",
    projectName: "nexus-web",
    status: "running",
    branch: "feature/dark-mode",
    commit: "f6e5d4c3b2a10987",
    commitMessage: "Add dark mode toggle to settings",
    triggeredBy: mockUsers[1],
    steps: makeSteps(["success", "success", "success", "running", "pending", "pending"]),
    duration: 0,
    startedAt: new Date(Date.now() - 1000 * 60 * 8).toISOString(),
    finishedAt: null,
    createdAt: new Date(Date.now() - 1000 * 60 * 8).toISOString(),
  },
  {
    id: "pipe-3",
    projectId: "proj-4",
    projectName: "data-pipeline",
    status: "failed",
    branch: "main",
    commit: "1a2b3c4d5e6f7890",
    commitMessage: "Update ETL batch processing configuration",
    triggeredBy: mockUsers[3],
    steps: makeSteps(["success", "success", "success", "failed", "skipped", "skipped"]),
    duration: 67,
    startedAt: new Date(Date.now() - 1000 * 60 * 60 * 3).toISOString(),
    finishedAt: new Date(Date.now() - 1000 * 60 * 60 * 3 + 67000).toISOString(),
    createdAt: new Date(Date.now() - 1000 * 60 * 60 * 3).toISOString(),
  },
  {
    id: "pipe-4",
    projectId: "proj-3",
    projectName: "auth-service",
    status: "success",
    branch: "release/v1.9",
    commit: "9f8e7d6c5b4a3210",
    commitMessage: "Bump version to 1.9.0",
    triggeredBy: mockUsers[2],
    steps: makeSteps(["success", "success", "success", "success", "success", "success"]),
    duration: 312,
    startedAt: new Date(Date.now() - 1000 * 60 * 60 * 5).toISOString(),
    finishedAt: new Date(Date.now() - 1000 * 60 * 60 * 5 + 312000).toISOString(),
    createdAt: new Date(Date.now() - 1000 * 60 * 60 * 5).toISOString(),
  },
  {
    id: "pipe-5",
    projectId: "proj-5",
    projectName: "notification-svc",
    status: "success",
    branch: "release/v2.0",
    commit: "0a1b2c3d4e5f6789",
    commitMessage: "Add Slack webhook integration",
    triggeredBy: mockUsers[1],
    steps: makeSteps(["success", "success", "success", "success", "success", "success"]),
    duration: 198,
    startedAt: new Date(Date.now() - 1000 * 60 * 60 * 8).toISOString(),
    finishedAt: new Date(Date.now() - 1000 * 60 * 60 * 8 + 198000).toISOString(),
    createdAt: new Date(Date.now() - 1000 * 60 * 60 * 8).toISOString(),
  },
  {
    id: "pipe-6",
    projectId: "proj-1",
    projectName: "nexus-api",
    status: "cancelled",
    branch: "feature/rate-limiting",
    commit: "5e4d3c2b1a098765",
    commitMessage: "WIP: Add rate limiting middleware",
    triggeredBy: mockUsers[0],
    steps: makeSteps(["success", "success", "skipped", "skipped", "skipped", "skipped"]),
    duration: 45,
    startedAt: new Date(Date.now() - 1000 * 60 * 60 * 12).toISOString(),
    finishedAt: new Date(Date.now() - 1000 * 60 * 60 * 12 + 45000).toISOString(),
    createdAt: new Date(Date.now() - 1000 * 60 * 60 * 12).toISOString(),
  },
];

function getStatusBadgeVariant(status: Status) {
  switch (status) {
    case "success": return "success" as const;
    case "running": return "info" as const;
    case "failed": return "error" as const;
    case "pending": return "warning" as const;
    case "cancelled": return "secondary" as const;
    default: return "secondary" as const;
  }
}

function getStatusIcon(status: Status) {
  switch (status) {
    case "success": return <CheckCircle2 size={16} className="text-nexus-green-400" />;
    case "running": return <Loader2 size={16} className="animate-spin text-nexus-blue-400" />;
    case "failed": return <XCircle size={16} className="text-nexus-red-400" />;
    case "cancelled": return <XCircle size={16} className="text-muted-foreground" />;
    default: return <Clock size={16} className="text-yellow-400" />;
  }
}

export default function PipelinesPage() {
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState<string>("all");

  const filteredPipelines = mockPipelines.filter((p) => {
    const matchesSearch =
      p.projectName.toLowerCase().includes(search.toLowerCase()) ||
      p.branch.toLowerCase().includes(search.toLowerCase()) ||
      p.commitMessage.toLowerCase().includes(search.toLowerCase());
    const matchesStatus = statusFilter === "all" || p.status === statusFilter;
    return matchesSearch && matchesStatus;
  });

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Pipelines</h1>
          <p className="text-sm text-muted-foreground">
            Monitor and manage your CI/CD pipeline runs.
          </p>
        </div>
        <Button size="sm">
          <Play className="mr-2 h-3.5 w-3.5" />
          Trigger Pipeline
        </Button>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-4">
        <div className="flex-1 max-w-md">
          <Input
            placeholder="Search pipelines..."
            icon={<Search size={16} />}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>
        <div className="flex items-center gap-2">
          {["all", "running", "success", "failed", "cancelled"].map((status) => (
            <Button
              key={status}
              variant={statusFilter === status ? "secondary" : "ghost"}
              size="sm"
              onClick={() => setStatusFilter(status)}
              className="capitalize"
            >
              {status}
            </Button>
          ))}
        </div>
      </div>

      {/* Pipeline Runs List */}
      <div className="space-y-3">
        {filteredPipelines.length === 0 ? (
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-16">
              <GitBranch size={40} className="mb-3 text-muted-foreground/30" />
              <p className="text-sm text-muted-foreground">No pipeline runs found</p>
            </CardContent>
          </Card>
        ) : (
          filteredPipelines.map((pipeline) => (
            <Card
              key={pipeline.id}
              className="transition-all duration-150 hover:border-primary/20 hover:shadow-sm"
            >
              <CardContent className="p-5">
                <div className="flex items-start justify-between">
                  {/* Left: Pipeline Info */}
                  <div className="flex items-start gap-4">
                    <div className="mt-0.5">{getStatusIcon(pipeline.status)}</div>
                    <div className="space-y-1.5">
                      <div className="flex items-center gap-2">
                        <span className="font-semibold">{pipeline.projectName}</span>
                        <Badge variant={getStatusBadgeVariant(pipeline.status)}>
                          {pipeline.status}
                        </Badge>
                      </div>
                      <p className="text-sm text-muted-foreground">
                        {pipeline.commitMessage}
                      </p>
                      <div className="flex items-center gap-4 text-xs text-muted-foreground">
                        <span className="flex items-center gap-1">
                          <GitBranch size={12} />
                          {pipeline.branch}
                        </span>
                        <span className="font-mono">
                          {truncateHash(pipeline.commit)}
                        </span>
                        <span className="flex items-center gap-1">
                          <User size={12} />
                          {pipeline.triggeredBy.name}
                        </span>
                        {pipeline.duration > 0 && (
                          <span className="flex items-center gap-1">
                            <Clock size={12} />
                            {formatDuration(pipeline.duration)}
                          </span>
                        )}
                        <span>{formatDate(pipeline.createdAt)}</span>
                      </div>
                      {/* Pipeline Steps Visualization */}
                      <div className="pt-2">
                        <PipelineStatus steps={pipeline.steps} compact />
                      </div>
                    </div>
                  </div>

                  {/* Right: Actions */}
                  <div className="flex items-center gap-1">
                    {pipeline.status === "failed" && (
                      <Button
                        variant="ghost"
                        size="sm"
                        className="text-xs"
                      >
                        <RotateCcw className="mr-1 h-3 w-3" />
                        Retry
                      </Button>
                    )}
                    {pipeline.status === "running" && (
                      <Button
                        variant="ghost"
                        size="sm"
                        className="text-xs text-nexus-red-400"
                      >
                        <XCircle className="mr-1 h-3 w-3" />
                        Cancel
                      </Button>
                    )}
                  </div>
                </div>
              </CardContent>
            </Card>
          ))
        )}
      </div>

      {/* Results count */}
      <p className="text-xs text-muted-foreground">
        Showing {filteredPipelines.length} of {mockPipelines.length} pipeline runs
      </p>
    </div>
  );
}
