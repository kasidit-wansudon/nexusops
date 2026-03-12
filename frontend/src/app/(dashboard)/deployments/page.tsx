"use client";

import { useState } from "react";
import {
  Rocket,
  Search,
  Filter,
  Plus,
  Globe,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent } from "@/components/ui/card";
import { DeploymentTable } from "@/components/deployment-table";
import { cn } from "@/lib/utils";
import type { Deployment, Environment } from "@/lib/types";

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
  { ...mockUser, id: "u2", name: "Sarah Chen", role: "developer" as const },
  { ...mockUser, id: "u3", name: "Mike Wilson", role: "developer" as const },
  { ...mockUser, id: "u4", name: "Alex Rivera", role: "developer" as const },
  { ...mockUser, id: "u5", name: "Emily Zhang", role: "developer" as const },
];

const mockDeployments: Deployment[] = [
  {
    id: "dep-1",
    projectId: "proj-1",
    projectName: "nexus-api",
    environment: "production",
    status: "success",
    version: "2.4.1",
    commit: "a1b2c3d4e5f6a7b8",
    commitMessage: "Fix auth token refresh logic",
    branch: "main",
    deployedBy: mockUsers[0],
    duration: 142,
    url: "https://api.nexusops.dev",
    rollbackAvailable: true,
    startedAt: new Date(Date.now() - 1000 * 60 * 15).toISOString(),
    finishedAt: new Date(Date.now() - 1000 * 60 * 13).toISOString(),
    createdAt: new Date(Date.now() - 1000 * 60 * 15).toISOString(),
  },
  {
    id: "dep-2",
    projectId: "proj-2",
    projectName: "nexus-web",
    environment: "staging",
    status: "running",
    version: "3.1.0-rc.2",
    commit: "f6e5d4c3b2a10987",
    commitMessage: "Add dark mode toggle to settings",
    branch: "feature/dark-mode",
    deployedBy: mockUsers[1],
    duration: 0,
    url: "https://staging.nexusops.dev",
    rollbackAvailable: false,
    startedAt: new Date(Date.now() - 1000 * 60 * 5).toISOString(),
    finishedAt: null,
    createdAt: new Date(Date.now() - 1000 * 60 * 5).toISOString(),
  },
  {
    id: "dep-3",
    projectId: "proj-3",
    projectName: "auth-service",
    environment: "production",
    status: "success",
    version: "1.8.3",
    commit: "c3d4e5f6a7b8c9d0",
    commitMessage: "Update RBAC permission checks",
    branch: "main",
    deployedBy: mockUsers[2],
    duration: 198,
    url: "https://auth.nexusops.dev",
    rollbackAvailable: true,
    startedAt: new Date(Date.now() - 1000 * 60 * 60 * 2).toISOString(),
    finishedAt: new Date(Date.now() - 1000 * 60 * 60 * 2 + 198000).toISOString(),
    createdAt: new Date(Date.now() - 1000 * 60 * 60 * 2).toISOString(),
  },
  {
    id: "dep-4",
    projectId: "proj-4",
    projectName: "data-pipeline",
    environment: "development",
    status: "failed",
    version: "0.5.0",
    commit: "d4e5f6a7b8c9d0e1",
    commitMessage: "Update ETL batch processing config",
    branch: "main",
    deployedBy: mockUsers[3],
    duration: 67,
    url: "",
    rollbackAvailable: false,
    startedAt: new Date(Date.now() - 1000 * 60 * 60 * 4).toISOString(),
    finishedAt: new Date(Date.now() - 1000 * 60 * 60 * 4 + 67000).toISOString(),
    createdAt: new Date(Date.now() - 1000 * 60 * 60 * 4).toISOString(),
  },
  {
    id: "dep-5",
    projectId: "proj-5",
    projectName: "notification-svc",
    environment: "staging",
    status: "success",
    version: "2.0.0",
    commit: "e5f6a7b8c9d0e1f2",
    commitMessage: "Add Slack webhook integration",
    branch: "release/v2.0",
    deployedBy: mockUsers[4],
    duration: 155,
    url: "https://staging-notify.nexusops.dev",
    rollbackAvailable: true,
    startedAt: new Date(Date.now() - 1000 * 60 * 60 * 6).toISOString(),
    finishedAt: new Date(Date.now() - 1000 * 60 * 60 * 6 + 155000).toISOString(),
    createdAt: new Date(Date.now() - 1000 * 60 * 60 * 6).toISOString(),
  },
  {
    id: "dep-6",
    projectId: "proj-1",
    projectName: "nexus-api",
    environment: "staging",
    status: "success",
    version: "2.4.1-rc.3",
    commit: "b2c3d4e5f6a7b8c9",
    commitMessage: "Add rate limiting middleware",
    branch: "main",
    deployedBy: mockUsers[1],
    duration: 128,
    url: "https://staging-api.nexusops.dev",
    rollbackAvailable: true,
    startedAt: new Date(Date.now() - 1000 * 60 * 60 * 12).toISOString(),
    finishedAt: new Date(Date.now() - 1000 * 60 * 60 * 12 + 128000).toISOString(),
    createdAt: new Date(Date.now() - 1000 * 60 * 60 * 12).toISOString(),
  },
  {
    id: "dep-7",
    projectId: "proj-6",
    projectName: "infra-terraform",
    environment: "production",
    status: "success",
    version: "1.2.0",
    commit: "f6a7b8c9d0e1f2a3",
    commitMessage: "Scale up ECS cluster capacity",
    branch: "main",
    deployedBy: mockUsers[0],
    duration: 340,
    url: "",
    rollbackAvailable: true,
    startedAt: new Date(Date.now() - 1000 * 60 * 60 * 24).toISOString(),
    finishedAt: new Date(Date.now() - 1000 * 60 * 60 * 24 + 340000).toISOString(),
    createdAt: new Date(Date.now() - 1000 * 60 * 60 * 24).toISOString(),
  },
];

const environments: Array<{ value: string; label: string; count: number }> = [
  { value: "all", label: "All Environments", count: mockDeployments.length },
  {
    value: "production",
    label: "Production",
    count: mockDeployments.filter((d) => d.environment === "production").length,
  },
  {
    value: "staging",
    label: "Staging",
    count: mockDeployments.filter((d) => d.environment === "staging").length,
  },
  {
    value: "development",
    label: "Development",
    count: mockDeployments.filter((d) => d.environment === "development").length,
  },
];

export default function DeploymentsPage() {
  const [envFilter, setEnvFilter] = useState("all");
  const [search, setSearch] = useState("");

  const filteredDeployments = mockDeployments.filter((d) => {
    const matchesEnv = envFilter === "all" || d.environment === envFilter;
    const matchesSearch =
      d.projectName.toLowerCase().includes(search.toLowerCase()) ||
      d.version.toLowerCase().includes(search.toLowerCase()) ||
      d.commitMessage.toLowerCase().includes(search.toLowerCase());
    return matchesEnv && matchesSearch;
  });

  const handleRollback = (id: string) => {
    // In production, call the rollback API
    console.log("Rolling back deployment:", id);
  };

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Deployments</h1>
          <p className="text-sm text-muted-foreground">
            Track and manage deployments across all environments.
          </p>
        </div>
        <Button size="sm">
          <Plus className="mr-2 h-3.5 w-3.5" />
          New Deployment
        </Button>
      </div>

      {/* Environment Filter Tabs */}
      <div className="flex items-center gap-2">
        {environments.map((env) => (
          <Button
            key={env.value}
            variant={envFilter === env.value ? "secondary" : "ghost"}
            size="sm"
            onClick={() => setEnvFilter(env.value)}
            className="gap-2"
          >
            {env.value !== "all" && (
              <Globe size={14} className={cn(
                env.value === "production" && "text-nexus-red-400",
                env.value === "staging" && "text-yellow-400",
                env.value === "development" && "text-nexus-blue-400",
              )} />
            )}
            {env.label}
            <span className="rounded-full bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
              {env.count}
            </span>
          </Button>
        ))}
      </div>

      {/* Search */}
      <div className="max-w-md">
        <Input
          placeholder="Search deployments..."
          icon={<Search size={16} />}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
      </div>

      {/* Deployments Table */}
      <DeploymentTable
        deployments={filteredDeployments}
        onRollback={handleRollback}
      />

      {/* Results count */}
      <p className="text-xs text-muted-foreground">
        Showing {filteredDeployments.length} of {mockDeployments.length} deployments
      </p>
    </div>
  );
}
