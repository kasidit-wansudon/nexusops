"use client";

import { useState } from "react";
import Link from "next/link";
import {
  FolderKanban,
  Rocket,
  GitBranch,
  Activity,
  ArrowRight,
  Clock,
  Plus,
  RefreshCw,
  TrendingUp,
} from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { StatsCard } from "@/components/metrics/stats-card";
import { formatDate, formatDuration, truncateHash } from "@/lib/utils";
import type { Status } from "@/lib/types";

// Mock data for the dashboard
const stats = {
  totalProjects: 24,
  activeDeployments: 18,
  pipelineSuccessRate: 94.2,
  uptime: 99.97,
  projectsTrend: 12,
  deploymentsTrend: 8,
  pipelinesTrend: 3.5,
  uptimeTrend: 0.02,
};

const recentDeployments = [
  {
    id: "dep-1",
    projectName: "nexus-api",
    environment: "production",
    version: "2.4.1",
    status: "success" as Status,
    deployedBy: "John Doe",
    startedAt: new Date(Date.now() - 1000 * 60 * 15).toISOString(),
    duration: 142,
  },
  {
    id: "dep-2",
    projectName: "nexus-web",
    environment: "staging",
    version: "3.1.0-rc.2",
    status: "running" as Status,
    deployedBy: "Sarah Chen",
    startedAt: new Date(Date.now() - 1000 * 60 * 5).toISOString(),
    duration: 0,
  },
  {
    id: "dep-3",
    projectName: "auth-service",
    environment: "production",
    version: "1.8.3",
    status: "success" as Status,
    deployedBy: "Mike Wilson",
    startedAt: new Date(Date.now() - 1000 * 60 * 60 * 2).toISOString(),
    duration: 198,
  },
  {
    id: "dep-4",
    projectName: "data-pipeline",
    environment: "development",
    version: "0.5.0",
    status: "failed" as Status,
    deployedBy: "Alex Rivera",
    startedAt: new Date(Date.now() - 1000 * 60 * 60 * 4).toISOString(),
    duration: 67,
  },
  {
    id: "dep-5",
    projectName: "notification-svc",
    environment: "staging",
    version: "2.0.0",
    status: "success" as Status,
    deployedBy: "Emily Zhang",
    startedAt: new Date(Date.now() - 1000 * 60 * 60 * 6).toISOString(),
    duration: 155,
  },
];

const recentPipelines = [
  {
    id: "pipe-1",
    projectName: "nexus-api",
    branch: "main",
    commit: "a1b2c3d4e5f6",
    status: "success" as Status,
    duration: 245,
    triggeredBy: "John Doe",
    createdAt: new Date(Date.now() - 1000 * 60 * 20).toISOString(),
  },
  {
    id: "pipe-2",
    projectName: "nexus-web",
    branch: "feature/dark-mode",
    commit: "f6e5d4c3b2a1",
    status: "running" as Status,
    duration: 0,
    triggeredBy: "Sarah Chen",
    createdAt: new Date(Date.now() - 1000 * 60 * 8).toISOString(),
  },
  {
    id: "pipe-3",
    projectName: "data-pipeline",
    branch: "main",
    commit: "1a2b3c4d5e6f",
    status: "failed" as Status,
    duration: 67,
    triggeredBy: "Alex Rivera",
    createdAt: new Date(Date.now() - 1000 * 60 * 60 * 3).toISOString(),
  },
  {
    id: "pipe-4",
    projectName: "auth-service",
    branch: "release/v1.9",
    commit: "9f8e7d6c5b4a",
    status: "success" as Status,
    duration: 312,
    triggeredBy: "Mike Wilson",
    createdAt: new Date(Date.now() - 1000 * 60 * 60 * 5).toISOString(),
  },
];

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

function getEnvColor(env: string) {
  switch (env) {
    case "production":
      return "text-nexus-red-400 bg-nexus-red-400/10 border-nexus-red-400/20";
    case "staging":
      return "text-yellow-400 bg-yellow-400/10 border-yellow-400/20";
    case "development":
      return "text-nexus-blue-400 bg-nexus-blue-400/10 border-nexus-blue-400/20";
    default:
      return "text-muted-foreground bg-muted";
  }
}

export default function DashboardPage() {
  const [refreshing, setRefreshing] = useState(false);

  const handleRefresh = () => {
    setRefreshing(true);
    setTimeout(() => setRefreshing(false), 1000);
  };

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Dashboard</h1>
          <p className="text-sm text-muted-foreground">
            Overview of your platform activity and health.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={handleRefresh}>
            <RefreshCw
              className={`mr-2 h-3.5 w-3.5 ${refreshing ? "animate-spin" : ""}`}
            />
            Refresh
          </Button>
          <Button size="sm">
            <Plus className="mr-2 h-3.5 w-3.5" />
            New Project
          </Button>
        </div>
      </div>

      {/* Stats Cards */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatsCard
          title="Total Projects"
          value={stats.totalProjects}
          icon={<FolderKanban className="h-5 w-5" />}
          trend={{ value: stats.projectsTrend, label: "from last month" }}
        />
        <StatsCard
          title="Active Deployments"
          value={stats.activeDeployments}
          icon={<Rocket className="h-5 w-5" />}
          trend={{ value: stats.deploymentsTrend, label: "from last month" }}
        />
        <StatsCard
          title="Pipeline Success Rate"
          value={`${stats.pipelineSuccessRate}%`}
          icon={<GitBranch className="h-5 w-5" />}
          trend={{ value: stats.pipelinesTrend, label: "from last week" }}
        />
        <StatsCard
          title="Platform Uptime"
          value={`${stats.uptime}%`}
          icon={<Activity className="h-5 w-5" />}
          trend={{ value: stats.uptimeTrend, label: "from last month" }}
        />
      </div>

      {/* Main Content Grid */}
      <div className="grid gap-6 lg:grid-cols-2">
        {/* Recent Deployments */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-4">
            <CardTitle className="text-base">Recent Deployments</CardTitle>
            <Link href="/deployments">
              <Button variant="ghost" size="sm">
                View all
                <ArrowRight className="ml-1 h-3.5 w-3.5" />
              </Button>
            </Link>
          </CardHeader>
          <CardContent className="p-0">
            <div className="divide-y divide-border/50">
              {recentDeployments.map((deploy) => (
                <div
                  key={deploy.id}
                  className="flex items-center justify-between px-6 py-3 transition-colors hover:bg-accent/30"
                >
                  <div className="flex items-center gap-3">
                    <div className="flex flex-col">
                      <span className="text-sm font-medium">
                        {deploy.projectName}
                      </span>
                      <div className="flex items-center gap-2 text-xs text-muted-foreground">
                        <span
                          className={`inline-flex items-center rounded border px-1.5 py-0.5 text-[10px] font-medium capitalize ${getEnvColor(deploy.environment)}`}
                        >
                          {deploy.environment}
                        </span>
                        <span>v{deploy.version}</span>
                      </div>
                    </div>
                  </div>
                  <div className="flex items-center gap-3">
                    <Badge variant={getStatusBadgeVariant(deploy.status)} dot>
                      {deploy.status}
                    </Badge>
                    <span className="text-xs text-muted-foreground">
                      {formatDate(deploy.startedAt)}
                    </span>
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>

        {/* Recent Pipeline Runs */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-4">
            <CardTitle className="text-base">Recent Pipelines</CardTitle>
            <Link href="/pipelines">
              <Button variant="ghost" size="sm">
                View all
                <ArrowRight className="ml-1 h-3.5 w-3.5" />
              </Button>
            </Link>
          </CardHeader>
          <CardContent className="p-0">
            <div className="divide-y divide-border/50">
              {recentPipelines.map((pipeline) => (
                <Link
                  key={pipeline.id}
                  href={`/pipelines/${pipeline.id}`}
                  className="flex items-center justify-between px-6 py-3 transition-colors hover:bg-accent/30"
                >
                  <div className="flex flex-col">
                    <span className="text-sm font-medium">
                      {pipeline.projectName}
                    </span>
                    <div className="flex items-center gap-2 text-xs text-muted-foreground">
                      <GitBranch className="h-3 w-3" />
                      <span>{pipeline.branch}</span>
                      <span className="font-mono">
                        {truncateHash(pipeline.commit)}
                      </span>
                    </div>
                  </div>
                  <div className="flex items-center gap-3">
                    <Badge variant={getStatusBadgeVariant(pipeline.status)} dot>
                      {pipeline.status}
                    </Badge>
                    {pipeline.duration > 0 && (
                      <div className="flex items-center gap-1 text-xs text-muted-foreground">
                        <Clock className="h-3 w-3" />
                        {formatDuration(pipeline.duration)}
                      </div>
                    )}
                  </div>
                </Link>
              ))}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Quick Actions */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Quick Actions</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <Link href="/projects">
              <Button
                variant="outline"
                className="w-full justify-start gap-3 h-auto py-4"
              >
                <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-nexus-blue-500/10 text-nexus-blue-400">
                  <Plus className="h-4 w-4" />
                </div>
                <div className="text-left">
                  <p className="text-sm font-medium">Create Project</p>
                  <p className="text-xs text-muted-foreground">
                    Set up a new project
                  </p>
                </div>
              </Button>
            </Link>
            <Link href="/pipelines">
              <Button
                variant="outline"
                className="w-full justify-start gap-3 h-auto py-4"
              >
                <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-nexus-green-500/10 text-nexus-green-400">
                  <GitBranch className="h-4 w-4" />
                </div>
                <div className="text-left">
                  <p className="text-sm font-medium">Trigger Pipeline</p>
                  <p className="text-xs text-muted-foreground">
                    Run a new build
                  </p>
                </div>
              </Button>
            </Link>
            <Link href="/deployments">
              <Button
                variant="outline"
                className="w-full justify-start gap-3 h-auto py-4"
              >
                <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-purple-500/10 text-purple-400">
                  <Rocket className="h-4 w-4" />
                </div>
                <div className="text-left">
                  <p className="text-sm font-medium">Deploy Now</p>
                  <p className="text-xs text-muted-foreground">
                    Ship to production
                  </p>
                </div>
              </Button>
            </Link>
            <Link href="/monitoring">
              <Button
                variant="outline"
                className="w-full justify-start gap-3 h-auto py-4"
              >
                <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-yellow-500/10 text-yellow-400">
                  <TrendingUp className="h-4 w-4" />
                </div>
                <div className="text-left">
                  <p className="text-sm font-medium">View Metrics</p>
                  <p className="text-xs text-muted-foreground">
                    Check platform health
                  </p>
                </div>
              </Button>
            </Link>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
