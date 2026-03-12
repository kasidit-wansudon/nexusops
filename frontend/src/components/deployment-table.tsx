"use client";

import {
  Rocket,
  RotateCcw,
  ExternalLink,
  GitCommit,
  GitBranch,
  Clock,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn, formatDate, formatDuration, truncateHash } from "@/lib/utils";
import type { Deployment, Environment } from "@/lib/types";

interface DeploymentTableProps {
  deployments: Deployment[];
  onRollback?: (id: string) => void;
}

function getStatusBadgeVariant(status: string) {
  switch (status) {
    case "success":
      return "success" as const;
    case "running":
      return "info" as const;
    case "failed":
      return "error" as const;
    case "pending":
      return "warning" as const;
    case "cancelled":
      return "secondary" as const;
    default:
      return "secondary" as const;
  }
}

function getEnvironmentColor(env: Environment): string {
  switch (env) {
    case "production":
      return "text-nexus-red-400 bg-nexus-red-400/10 border-nexus-red-400/20";
    case "staging":
      return "text-yellow-400 bg-yellow-400/10 border-yellow-400/20";
    case "development":
      return "text-nexus-blue-400 bg-nexus-blue-400/10 border-nexus-blue-400/20";
    case "preview":
      return "text-purple-400 bg-purple-400/10 border-purple-400/20";
    default:
      return "text-muted-foreground bg-muted border-border";
  }
}

export function DeploymentTable({
  deployments,
  onRollback,
}: DeploymentTableProps) {
  if (deployments.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center rounded-xl border border-border bg-card py-16">
        <Rocket size={40} className="mb-3 text-muted-foreground/30" />
        <p className="text-sm text-muted-foreground">No deployments found</p>
      </div>
    );
  }

  return (
    <div className="overflow-hidden rounded-xl border border-border bg-card">
      <table className="w-full">
        <thead>
          <tr className="border-b border-border bg-muted/30">
            <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-muted-foreground">
              Deployment
            </th>
            <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-muted-foreground">
              Environment
            </th>
            <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-muted-foreground">
              Status
            </th>
            <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-muted-foreground">
              Commit
            </th>
            <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-muted-foreground">
              Duration
            </th>
            <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-muted-foreground">
              Deployed By
            </th>
            <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-muted-foreground">
              Time
            </th>
            <th className="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-muted-foreground">
              Actions
            </th>
          </tr>
        </thead>
        <tbody className="divide-y divide-border/50">
          {deployments.map((deployment) => (
            <tr
              key={deployment.id}
              className="transition-colors hover:bg-accent/30"
            >
              {/* Deployment Info */}
              <td className="px-4 py-3">
                <div>
                  <p className="text-sm font-medium">{deployment.projectName}</p>
                  <p className="text-xs text-muted-foreground">
                    v{deployment.version}
                  </p>
                </div>
              </td>

              {/* Environment */}
              <td className="px-4 py-3">
                <span
                  className={cn(
                    "inline-flex items-center rounded-md border px-2 py-0.5 text-xs font-medium capitalize",
                    getEnvironmentColor(deployment.environment)
                  )}
                >
                  {deployment.environment}
                </span>
              </td>

              {/* Status */}
              <td className="px-4 py-3">
                <Badge
                  variant={getStatusBadgeVariant(deployment.status)}
                  dot
                >
                  {deployment.status}
                </Badge>
              </td>

              {/* Commit */}
              <td className="px-4 py-3">
                <div className="flex items-center gap-2">
                  <div className="flex items-center gap-1 text-xs font-mono text-muted-foreground">
                    <GitCommit size={12} />
                    {truncateHash(deployment.commit)}
                  </div>
                  <div className="flex items-center gap-1 text-xs text-muted-foreground">
                    <GitBranch size={12} />
                    {deployment.branch}
                  </div>
                </div>
              </td>

              {/* Duration */}
              <td className="px-4 py-3">
                <div className="flex items-center gap-1 text-xs text-muted-foreground">
                  <Clock size={12} />
                  {formatDuration(deployment.duration)}
                </div>
              </td>

              {/* Deployed By */}
              <td className="px-4 py-3">
                <div className="flex items-center gap-2">
                  <div className="flex h-6 w-6 items-center justify-center rounded-full bg-accent text-[10px] font-medium">
                    {deployment.deployedBy.name
                      .split(" ")
                      .map((n) => n[0])
                      .join("")}
                  </div>
                  <span className="text-xs text-muted-foreground">
                    {deployment.deployedBy.name}
                  </span>
                </div>
              </td>

              {/* Time */}
              <td className="px-4 py-3">
                <span className="text-xs text-muted-foreground">
                  {formatDate(deployment.startedAt)}
                </span>
              </td>

              {/* Actions */}
              <td className="px-4 py-3 text-right">
                <div className="flex items-center justify-end gap-1">
                  {deployment.url && (
                    <a
                      href={deployment.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      title="Open deployment URL"
                      className="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                    >
                      <ExternalLink size={14} />
                    </a>
                  )}
                  {deployment.rollbackAvailable &&
                    deployment.status === "success" && (
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        onClick={() => onRollback?.(deployment.id)}
                        title="Rollback to this version"
                        className="text-muted-foreground hover:text-yellow-400"
                      >
                        <RotateCcw size={14} />
                      </Button>
                    )}
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
