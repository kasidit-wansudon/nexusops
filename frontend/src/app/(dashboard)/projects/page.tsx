"use client";

import { useState } from "react";
import { Plus, Search, LayoutGrid, List, Filter, FolderKanban } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ProjectCard } from "@/components/project-card";
import type { Project, User, HealthStatus } from "@/lib/types";

// Mock team members
const mockUsers: User[] = [
  {
    id: "u1",
    name: "John Doe",
    email: "john@nexusops.dev",
    avatarUrl: "",
    role: "admin",
    lastActive: new Date().toISOString(),
    createdAt: "2024-01-01T00:00:00Z",
  },
  {
    id: "u2",
    name: "Sarah Chen",
    email: "sarah@nexusops.dev",
    avatarUrl: "",
    role: "developer",
    lastActive: new Date().toISOString(),
    createdAt: "2024-01-15T00:00:00Z",
  },
  {
    id: "u3",
    name: "Mike Wilson",
    email: "mike@nexusops.dev",
    avatarUrl: "",
    role: "developer",
    lastActive: new Date().toISOString(),
    createdAt: "2024-02-01T00:00:00Z",
  },
];

// Mock projects
const mockProjects: Project[] = [
  {
    id: "proj-1",
    name: "nexus-api",
    description: "Core REST API service powering the NexusOps platform. Built with Go and PostgreSQL.",
    repository: "github.com/nexusops/nexus-api",
    language: "Go",
    framework: "Fiber",
    status: "success",
    lastDeployedAt: new Date(Date.now() - 1000 * 60 * 15).toISOString(),
    lastDeployedBy: mockUsers[0],
    team: mockUsers,
    environment: "production",
    branch: "main",
    healthStatus: "healthy" as HealthStatus,
    createdAt: "2024-01-10T00:00:00Z",
    updatedAt: new Date().toISOString(),
  },
  {
    id: "proj-2",
    name: "nexus-web",
    description: "Frontend dashboard application for NexusOps. Next.js 14 with TypeScript and Tailwind.",
    repository: "github.com/nexusops/nexus-web",
    language: "TypeScript",
    framework: "Next.js",
    status: "running",
    lastDeployedAt: new Date(Date.now() - 1000 * 60 * 5).toISOString(),
    lastDeployedBy: mockUsers[1],
    team: [mockUsers[0], mockUsers[1]],
    environment: "staging",
    branch: "feature/dark-mode",
    healthStatus: "healthy" as HealthStatus,
    createdAt: "2024-01-12T00:00:00Z",
    updatedAt: new Date().toISOString(),
  },
  {
    id: "proj-3",
    name: "auth-service",
    description: "Authentication and authorization microservice. OAuth 2.0, JWT tokens, RBAC.",
    repository: "github.com/nexusops/auth-service",
    language: "TypeScript",
    framework: "NestJS",
    status: "success",
    lastDeployedAt: new Date(Date.now() - 1000 * 60 * 60 * 2).toISOString(),
    lastDeployedBy: mockUsers[2],
    team: mockUsers,
    environment: "production",
    branch: "main",
    healthStatus: "healthy" as HealthStatus,
    createdAt: "2024-01-15T00:00:00Z",
    updatedAt: new Date().toISOString(),
  },
  {
    id: "proj-4",
    name: "data-pipeline",
    description: "ETL data pipeline for analytics and reporting. Processes 10M+ events per day.",
    repository: "github.com/nexusops/data-pipeline",
    language: "Python",
    framework: "Apache Beam",
    status: "failed",
    lastDeployedAt: new Date(Date.now() - 1000 * 60 * 60 * 4).toISOString(),
    lastDeployedBy: mockUsers[0],
    team: [mockUsers[0], mockUsers[2]],
    environment: "development",
    branch: "main",
    healthStatus: "degraded" as HealthStatus,
    createdAt: "2024-02-01T00:00:00Z",
    updatedAt: new Date().toISOString(),
  },
  {
    id: "proj-5",
    name: "notification-svc",
    description: "Push notification and email delivery service. Supports Slack, email, SMS, and webhooks.",
    repository: "github.com/nexusops/notification-svc",
    language: "Go",
    framework: "Chi",
    status: "success",
    lastDeployedAt: new Date(Date.now() - 1000 * 60 * 60 * 6).toISOString(),
    lastDeployedBy: mockUsers[1],
    team: [mockUsers[1]],
    environment: "staging",
    branch: "release/v2.0",
    healthStatus: "healthy" as HealthStatus,
    createdAt: "2024-02-10T00:00:00Z",
    updatedAt: new Date().toISOString(),
  },
  {
    id: "proj-6",
    name: "infra-terraform",
    description: "Infrastructure as code repository. AWS resources managed via Terraform modules.",
    repository: "github.com/nexusops/infra-terraform",
    language: "Go",
    framework: "Terraform",
    status: "success",
    lastDeployedAt: new Date(Date.now() - 1000 * 60 * 60 * 24).toISOString(),
    lastDeployedBy: mockUsers[0],
    team: [mockUsers[0]],
    environment: "production",
    branch: "main",
    healthStatus: "healthy" as HealthStatus,
    createdAt: "2024-01-05T00:00:00Z",
    updatedAt: new Date().toISOString(),
  },
];

export default function ProjectsPage() {
  const [search, setSearch] = useState("");
  const [view, setView] = useState<"grid" | "list">("grid");

  const filteredProjects = mockProjects.filter(
    (p) =>
      p.name.toLowerCase().includes(search.toLowerCase()) ||
      p.description.toLowerCase().includes(search.toLowerCase()) ||
      p.language.toLowerCase().includes(search.toLowerCase())
  );

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Projects</h1>
          <p className="text-sm text-muted-foreground">
            Manage and monitor your projects across all environments.
          </p>
        </div>
        <Button size="sm">
          <Plus className="mr-2 h-3.5 w-3.5" />
          New Project
        </Button>
      </div>

      {/* Toolbar */}
      <div className="flex items-center justify-between gap-4">
        <div className="flex-1 max-w-md">
          <Input
            placeholder="Search projects..."
            icon={<Search size={16} />}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm">
            <Filter className="mr-2 h-3.5 w-3.5" />
            Filters
          </Button>
          <div className="flex items-center rounded-md border border-border">
            <Button
              variant={view === "grid" ? "secondary" : "ghost"}
              size="icon-sm"
              onClick={() => setView("grid")}
              className="rounded-r-none"
            >
              <LayoutGrid className="h-3.5 w-3.5" />
            </Button>
            <Button
              variant={view === "list" ? "secondary" : "ghost"}
              size="icon-sm"
              onClick={() => setView("list")}
              className="rounded-l-none"
            >
              <List className="h-3.5 w-3.5" />
            </Button>
          </div>
        </div>
      </div>

      {/* Projects Grid */}
      {filteredProjects.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-xl border border-border bg-card py-16">
          <FolderKanban size={40} className="mb-3 text-muted-foreground/30" />
          <p className="text-sm text-muted-foreground">No projects found</p>
          <p className="text-xs text-muted-foreground/70 mt-1">
            Try adjusting your search or filters
          </p>
        </div>
      ) : (
        <div
          className={
            view === "grid"
              ? "grid gap-4 sm:grid-cols-2 lg:grid-cols-3"
              : "flex flex-col gap-3"
          }
        >
          {filteredProjects.map((project) => (
            <ProjectCard key={project.id} project={project} />
          ))}
        </div>
      )}

      {/* Results count */}
      <p className="text-xs text-muted-foreground">
        Showing {filteredProjects.length} of {mockProjects.length} projects
      </p>
    </div>
  );
}
