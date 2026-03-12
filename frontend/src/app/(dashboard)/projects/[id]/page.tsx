"use client";

import { useState } from "react";
import Link from "next/link";
import {
  ArrowLeft,
  GitBranch,
  Globe,
  Clock,
  Users,
  Rocket,
  Settings,
  ExternalLink,
  Activity,
  MoreHorizontal,
  Plus,
  Circle,
} from "lucide-react";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn, formatDate, formatDuration, statusDot } from "@/lib/utils";
import type { Status, HealthStatus as HealthStatusType } from "@/lib/types";

// Mock data for the project detail
const project = {
  id: "proj-1",
  name: "nexus-api",
  description:
    "Core REST API service powering the NexusOps platform. Built with Go and PostgreSQL.",
  repository: "github.com/nexusops/nexus-api",
  language: "Go",
  framework: "Fiber",
  status: "success" as Status,
  healthStatus: "healthy" as HealthStatusType,
  branch: "main",
  lastDeployedAt: new Date(Date.now() - 1000 * 60 * 15).toISOString(),
};

const environments = [
  {
    name: "Production",
    status: "healthy" as HealthStatusType,
    version: "v2.4.1",
    url: "https://api.nexusops.dev",
    lastDeployed: new Date(Date.now() - 1000 * 60 * 15).toISOString(),
    replicas: 3,
  },
  {
    name: "Staging",
    status: "healthy" as HealthStatusType,
    version: "v2.5.0-rc.1",
    url: "https://staging-api.nexusops.dev",
    lastDeployed: new Date(Date.now() - 1000 * 60 * 60 * 2).toISOString(),
    replicas: 2,
  },
  {
    name: "Development",
    status: "degraded" as HealthStatusType,
    version: "v2.5.0-dev.47",
    url: "https://dev-api.nexusops.dev",
    lastDeployed: new Date(Date.now() - 1000 * 60 * 60 * 6).toISOString(),
    replicas: 1,
  },
];

const recentActivity = [
  {
    id: "act-1",
    type: "deployment",
    message: "Deployed v2.4.1 to production",
    user: "John Doe",
    time: new Date(Date.now() - 1000 * 60 * 15).toISOString(),
    status: "success" as Status,
  },
  {
    id: "act-2",
    type: "pipeline",
    message: "Pipeline #847 completed on main",
    user: "John Doe",
    time: new Date(Date.now() - 1000 * 60 * 20).toISOString(),
    status: "success" as Status,
  },
  {
    id: "act-3",
    type: "deployment",
    message: "Deployed v2.5.0-rc.1 to staging",
    user: "Sarah Chen",
    time: new Date(Date.now() - 1000 * 60 * 60 * 2).toISOString(),
    status: "success" as Status,
  },
  {
    id: "act-4",
    type: "pipeline",
    message: "Pipeline #845 failed on feature/auth-v2",
    user: "Mike Wilson",
    time: new Date(Date.now() - 1000 * 60 * 60 * 4).toISOString(),
    status: "failed" as Status,
  },
  {
    id: "act-5",
    type: "config",
    message: "Updated environment variables for staging",
    user: "Sarah Chen",
    time: new Date(Date.now() - 1000 * 60 * 60 * 8).toISOString(),
    status: "success" as Status,
  },
];

const recentPipelines = [
  {
    id: "pipe-1",
    branch: "main",
    commit: "a1b2c3d",
    commitMessage: "fix: resolve race condition in connection pool",
    status: "success" as Status,
    duration: 245,
    triggeredBy: "John Doe",
    createdAt: new Date(Date.now() - 1000 * 60 * 20).toISOString(),
  },
  {
    id: "pipe-2",
    branch: "feature/auth-v2",
    commit: "f6e5d4c",
    commitMessage: "feat: add OAuth2 PKCE flow support",
    status: "failed" as Status,
    duration: 122,
    triggeredBy: "Mike Wilson",
    createdAt: new Date(Date.now() - 1000 * 60 * 60 * 4).toISOString(),
  },
  {
    id: "pipe-3",
    branch: "main",
    commit: "9c8b7a6",
    commitMessage: "chore: update dependencies to latest versions",
    status: "success" as Status,
    duration: 198,
    triggeredBy: "Sarah Chen",
    createdAt: new Date(Date.now() - 1000 * 60 * 60 * 12).toISOString(),
  },
];

const deployments = [
  {
    id: "dep-1",
    environment: "production",
    version: "v2.4.1",
    status: "success" as Status,
    deployedBy: "John Doe",
    time: new Date(Date.now() - 1000 * 60 * 15).toISOString(),
    duration: 142,
  },
  {
    id: "dep-2",
    environment: "staging",
    version: "v2.5.0-rc.1",
    status: "success" as Status,
    deployedBy: "Sarah Chen",
    time: new Date(Date.now() - 1000 * 60 * 60 * 2).toISOString(),
    duration: 98,
  },
  {
    id: "dep-3",
    environment: "production",
    version: "v2.4.0",
    status: "success" as Status,
    deployedBy: "John Doe",
    time: new Date(Date.now() - 1000 * 60 * 60 * 24 * 3).toISOString(),
    duration: 155,
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
    default:
      return "secondary" as const;
  }
}

function getHealthColor(status: HealthStatusType) {
  switch (status) {
    case "healthy":
      return "text-nexus-green-400";
    case "degraded":
      return "text-yellow-400";
    case "down":
      return "text-nexus-red-400";
  }
}

export default function ProjectDetailPage() {
  const [activeTab, setActiveTab] = useState("overview");

  return (
    <div className="space-y-6">
      {/* Breadcrumb & Header */}
      <div>
        <Link
          href="/projects"
          className="mb-4 inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground transition-colors"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          Back to Projects
        </Link>

        <div className="flex items-start justify-between">
          <div>
            <div className="flex items-center gap-3">
              <h1 className="text-2xl font-bold tracking-tight">
                {project.name}
              </h1>
              <Badge variant={getStatusBadgeVariant(project.status)} dot>
                {project.status}
              </Badge>
              <div className="flex items-center gap-1.5">
                <Circle
                  size={8}
                  className={cn(
                    "fill-current",
                    statusDot(project.healthStatus)
                  )}
                />
                <span
                  className={cn(
                    "text-xs capitalize",
                    getHealthColor(project.healthStatus)
                  )}
                >
                  {project.healthStatus}
                </span>
              </div>
            </div>
            <p className="mt-1 text-sm text-muted-foreground">
              {project.description}
            </p>
            <div className="mt-3 flex items-center gap-4 text-xs text-muted-foreground">
              <a
                href={`https://${project.repository}`}
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-1 hover:text-foreground transition-colors"
              >
                <GitBranch className="h-3 w-3" />
                {project.repository}
                <ExternalLink className="h-2.5 w-2.5" />
              </a>
              <span className="flex items-center gap-1">
                <Clock className="h-3 w-3" />
                Last deployed {formatDate(project.lastDeployedAt)}
              </span>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm">
              <Settings className="mr-2 h-3.5 w-3.5" />
              Settings
            </Button>
            <Button size="sm">
              <Rocket className="mr-2 h-3.5 w-3.5" />
              Deploy
            </Button>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="environments">Environments</TabsTrigger>
          <TabsTrigger value="pipelines">Pipelines</TabsTrigger>
          <TabsTrigger value="deployments">Deployments</TabsTrigger>
          <TabsTrigger value="settings">Settings</TabsTrigger>
        </TabsList>

        {/* Overview Tab */}
        <TabsContent value="overview">
          <div className="grid gap-6 lg:grid-cols-3">
            {/* Current Deployment Status */}
            <Card className="lg:col-span-2">
              <CardHeader>
                <CardTitle className="text-base">Current Status</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="grid gap-4 sm:grid-cols-3">
                  {environments.map((env) => (
                    <div
                      key={env.name}
                      className="rounded-lg border border-border p-4"
                    >
                      <div className="flex items-center justify-between mb-2">
                        <span className="text-sm font-medium">{env.name}</span>
                        <div className="flex items-center gap-1">
                          <Circle
                            size={6}
                            className={cn(
                              "fill-current",
                              statusDot(env.status)
                            )}
                          />
                          <span
                            className={cn(
                              "text-xs capitalize",
                              getHealthColor(env.status)
                            )}
                          >
                            {env.status}
                          </span>
                        </div>
                      </div>
                      <p className="text-lg font-bold">{env.version}</p>
                      <div className="mt-2 flex items-center justify-between text-xs text-muted-foreground">
                        <span>{env.replicas} replicas</span>
                        <span>{formatDate(env.lastDeployed)}</span>
                      </div>
                      {env.url && (
                        <a
                          href={env.url}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="mt-2 flex items-center gap-1 text-xs text-primary hover:underline"
                        >
                          <Globe className="h-3 w-3" />
                          {env.url.replace("https://", "")}
                        </a>
                      )}
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>

            {/* Recent Activity */}
            <Card>
              <CardHeader className="flex flex-row items-center justify-between">
                <CardTitle className="text-base">Recent Activity</CardTitle>
                <Button variant="ghost" size="icon-sm">
                  <MoreHorizontal className="h-4 w-4" />
                </Button>
              </CardHeader>
              <CardContent className="p-0">
                <div className="divide-y divide-border/50">
                  {recentActivity.map((activity) => (
                    <div key={activity.id} className="px-6 py-3">
                      <div className="flex items-start gap-2">
                        <div
                          className={cn(
                            "mt-1 h-1.5 w-1.5 rounded-full shrink-0",
                            activity.status === "success"
                              ? "bg-nexus-green-400"
                              : activity.status === "failed"
                                ? "bg-nexus-red-400"
                                : "bg-nexus-blue-400"
                          )}
                        />
                        <div>
                          <p className="text-xs text-foreground">
                            {activity.message}
                          </p>
                          <p className="text-[10px] text-muted-foreground mt-0.5">
                            {activity.user} &middot;{" "}
                            {formatDate(activity.time)}
                          </p>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        {/* Environments Tab */}
        <TabsContent value="environments">
          <div className="grid gap-4 sm:grid-cols-3">
            {environments.map((env) => (
              <Card key={env.name}>
                <CardContent className="p-6">
                  <div className="flex items-center justify-between mb-4">
                    <h3 className="text-lg font-semibold">{env.name}</h3>
                    <div className="flex items-center gap-1.5">
                      <Circle
                        size={8}
                        className={cn(
                          "fill-current",
                          statusDot(env.status)
                        )}
                      />
                      <span
                        className={cn(
                          "text-sm capitalize",
                          getHealthColor(env.status)
                        )}
                      >
                        {env.status}
                      </span>
                    </div>
                  </div>
                  <div className="space-y-3">
                    <div className="flex items-center justify-between text-sm">
                      <span className="text-muted-foreground">Version</span>
                      <span className="font-mono font-medium">{env.version}</span>
                    </div>
                    <div className="flex items-center justify-between text-sm">
                      <span className="text-muted-foreground">Replicas</span>
                      <span className="font-medium">{env.replicas}</span>
                    </div>
                    <div className="flex items-center justify-between text-sm">
                      <span className="text-muted-foreground">
                        Last Deployed
                      </span>
                      <span>{formatDate(env.lastDeployed)}</span>
                    </div>
                    {env.url && (
                      <div className="flex items-center justify-between text-sm">
                        <span className="text-muted-foreground">URL</span>
                        <a
                          href={env.url}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="flex items-center gap-1 text-primary hover:underline"
                        >
                          <Globe className="h-3 w-3" />
                          Visit
                          <ExternalLink className="h-2.5 w-2.5" />
                        </a>
                      </div>
                    )}
                  </div>
                  <div className="mt-4 flex gap-2">
                    <Button variant="outline" size="sm" className="flex-1">
                      Rollback
                    </Button>
                    <Button size="sm" className="flex-1">
                      Deploy
                    </Button>
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        </TabsContent>

        {/* Pipelines Tab */}
        <TabsContent value="pipelines">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <CardTitle className="text-base">Pipeline Runs</CardTitle>
              <Button size="sm">
                <Plus className="mr-2 h-3.5 w-3.5" />
                Trigger Pipeline
              </Button>
            </CardHeader>
            <CardContent className="p-0">
              <div className="divide-y divide-border/50">
                {recentPipelines.map((pipeline) => (
                  <Link
                    key={pipeline.id}
                    href={`/pipelines/${pipeline.id}`}
                    className="flex items-center justify-between px-6 py-4 transition-colors hover:bg-accent/30"
                  >
                    <div className="flex flex-col gap-1">
                      <span className="text-sm font-medium font-mono">
                        {pipeline.commitMessage}
                      </span>
                      <div className="flex items-center gap-3 text-xs text-muted-foreground">
                        <span className="flex items-center gap-1">
                          <GitBranch className="h-3 w-3" />
                          {pipeline.branch}
                        </span>
                        <span className="font-mono">{pipeline.commit}</span>
                        <span>{pipeline.triggeredBy}</span>
                      </div>
                    </div>
                    <div className="flex items-center gap-3">
                      <Badge variant={getStatusBadgeVariant(pipeline.status)} dot>
                        {pipeline.status}
                      </Badge>
                      <div className="flex items-center gap-1 text-xs text-muted-foreground">
                        <Clock className="h-3 w-3" />
                        {formatDuration(pipeline.duration)}
                      </div>
                    </div>
                  </Link>
                ))}
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        {/* Deployments Tab */}
        <TabsContent value="deployments">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Deployment History</CardTitle>
            </CardHeader>
            <CardContent className="p-0">
              <div className="divide-y divide-border/50">
                {deployments.map((deploy) => (
                  <div
                    key={deploy.id}
                    className="flex items-center justify-between px-6 py-4"
                  >
                    <div className="flex flex-col gap-1">
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-medium">
                          {deploy.version}
                        </span>
                        <span className="inline-flex items-center rounded border border-border px-1.5 py-0.5 text-[10px] font-medium capitalize text-muted-foreground">
                          {deploy.environment}
                        </span>
                      </div>
                      <span className="text-xs text-muted-foreground">
                        by {deploy.deployedBy} &middot; {formatDate(deploy.time)}
                      </span>
                    </div>
                    <div className="flex items-center gap-3">
                      <Badge variant={getStatusBadgeVariant(deploy.status)} dot>
                        {deploy.status}
                      </Badge>
                      <span className="text-xs text-muted-foreground">
                        {formatDuration(deploy.duration)}
                      </span>
                    </div>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        {/* Settings Tab */}
        <TabsContent value="settings">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Project Settings</CardTitle>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="grid gap-4 sm:grid-cols-2">
                <div>
                  <label className="text-sm font-medium">Project Name</label>
                  <p className="mt-1 text-sm text-muted-foreground">
                    {project.name}
                  </p>
                </div>
                <div>
                  <label className="text-sm font-medium">Repository</label>
                  <p className="mt-1 text-sm text-muted-foreground">
                    {project.repository}
                  </p>
                </div>
                <div>
                  <label className="text-sm font-medium">Default Branch</label>
                  <p className="mt-1 text-sm text-muted-foreground">
                    {project.branch}
                  </p>
                </div>
                <div>
                  <label className="text-sm font-medium">Language</label>
                  <p className="mt-1 text-sm text-muted-foreground">
                    {project.language} / {project.framework}
                  </p>
                </div>
              </div>
              <div className="border-t border-border pt-6">
                <h4 className="text-sm font-medium text-nexus-red-400">
                  Danger Zone
                </h4>
                <p className="mt-1 text-xs text-muted-foreground">
                  These actions are irreversible. Please be careful.
                </p>
                <div className="mt-4 flex gap-2">
                  <Button variant="outline" size="sm">
                    Archive Project
                  </Button>
                  <Button variant="destructive" size="sm">
                    Delete Project
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
