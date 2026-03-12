import Link from "next/link";
import {
  GitBranch,
  Clock,
  Users,
  ExternalLink,
  Circle,
} from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { cn, formatDate, statusDot } from "@/lib/utils";
import type { Project } from "@/lib/types";

interface ProjectCardProps {
  project: Project;
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
    default:
      return "secondary" as const;
  }
}

function getLanguageColor(language: string): string {
  const colors: Record<string, string> = {
    TypeScript: "bg-nexus-blue-400",
    JavaScript: "bg-yellow-400",
    Python: "bg-nexus-green-400",
    Go: "bg-cyan-400",
    Rust: "bg-orange-400",
    Java: "bg-red-400",
    Ruby: "bg-red-500",
    "C#": "bg-purple-400",
  };
  return colors[language] || "bg-nexus-gray-400";
}

export function ProjectCard({ project }: ProjectCardProps) {
  return (
    <Link href={`/projects/${project.id}`}>
      <Card className="group cursor-pointer transition-all duration-200 hover:border-primary/30 hover:shadow-md hover:shadow-primary/5">
        <CardContent className="p-5">
          {/* Header */}
          <div className="mb-3 flex items-start justify-between">
            <div className="flex-1">
              <div className="flex items-center gap-2">
                <h3 className="font-semibold text-foreground group-hover:text-primary transition-colors">
                  {project.name}
                </h3>
                <ExternalLink
                  size={12}
                  className="text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100"
                />
              </div>
              <p className="mt-1 text-sm text-muted-foreground line-clamp-2">
                {project.description}
              </p>
            </div>
            <Badge variant={getStatusBadgeVariant(project.status)} dot>
              {project.status}
            </Badge>
          </div>

          {/* Language & Framework */}
          <div className="mb-4 flex items-center gap-3">
            <div className="flex items-center gap-1.5">
              <span
                className={cn(
                  "h-2.5 w-2.5 rounded-full",
                  getLanguageColor(project.language)
                )}
              />
              <span className="text-xs text-muted-foreground">
                {project.language}
              </span>
            </div>
            {project.framework && (
              <span className="text-xs text-muted-foreground">
                {project.framework}
              </span>
            )}
          </div>

          {/* Footer info */}
          <div className="flex items-center justify-between border-t border-border/50 pt-3">
            <div className="flex items-center gap-4">
              <div className="flex items-center gap-1 text-xs text-muted-foreground">
                <GitBranch size={12} />
                <span>{project.branch}</span>
              </div>
              <div className="flex items-center gap-1 text-xs text-muted-foreground">
                <Clock size={12} />
                <span>{formatDate(project.lastDeployedAt)}</span>
              </div>
            </div>

            {/* Team avatars */}
            <div className="flex items-center gap-1">
              <div className="flex -space-x-1.5">
                {project.team.slice(0, 3).map((member) => (
                  <div
                    key={member.id}
                    className="flex h-6 w-6 items-center justify-center rounded-full border-2 border-card bg-accent text-[10px] font-medium"
                    title={member.name}
                  >
                    {member.name
                      .split(" ")
                      .map((n) => n[0])
                      .join("")}
                  </div>
                ))}
              </div>
              {project.team.length > 3 && (
                <span className="text-xs text-muted-foreground">
                  +{project.team.length - 3}
                </span>
              )}
            </div>
          </div>

          {/* Health indicator */}
          <div className="mt-3 flex items-center gap-1.5">
            <Circle
              size={6}
              className={cn(
                "fill-current",
                statusDot(project.healthStatus)
              )}
            />
            <span className="text-xs capitalize text-muted-foreground">
              {project.healthStatus}
            </span>
          </div>
        </CardContent>
      </Card>
    </Link>
  );
}
