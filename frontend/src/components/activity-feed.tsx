import {
  Rocket,
  GitBranch,
  FolderPlus,
  UserPlus,
  AlertTriangle,
  RotateCcw,
  Settings2,
} from "lucide-react";
import { cn, formatDate } from "@/lib/utils";
import type { ActivityItem } from "@/lib/types";

interface ActivityFeedProps {
  activities: ActivityItem[];
}

function getActivityIcon(type: ActivityItem["type"]) {
  switch (type) {
    case "deployment":
      return <Rocket size={14} />;
    case "pipeline":
      return <GitBranch size={14} />;
    case "project":
      return <FolderPlus size={14} />;
    case "team":
      return <UserPlus size={14} />;
    case "alert":
      return <AlertTriangle size={14} />;
    case "rollback":
      return <RotateCcw size={14} />;
    case "config":
      return <Settings2 size={14} />;
    default:
      return <GitBranch size={14} />;
  }
}

function getActivityIconBg(type: ActivityItem["type"]): string {
  switch (type) {
    case "deployment":
      return "bg-nexus-blue-400/10 text-nexus-blue-400";
    case "pipeline":
      return "bg-nexus-green-400/10 text-nexus-green-400";
    case "project":
      return "bg-purple-400/10 text-purple-400";
    case "team":
      return "bg-cyan-400/10 text-cyan-400";
    case "alert":
      return "bg-nexus-red-400/10 text-nexus-red-400";
    case "rollback":
      return "bg-yellow-400/10 text-yellow-400";
    case "config":
      return "bg-nexus-gray-400/10 text-nexus-gray-400";
    default:
      return "bg-muted text-muted-foreground";
  }
}

export function ActivityFeed({ activities }: ActivityFeedProps) {
  if (activities.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-12">
        <GitBranch size={32} className="mb-2 text-muted-foreground/30" />
        <p className="text-sm text-muted-foreground">No recent activity</p>
      </div>
    );
  }

  return (
    <div className="space-y-0">
      {activities.map((activity, index) => (
        <div
          key={activity.id}
          className="group relative flex gap-3 py-3"
        >
          {/* Timeline line */}
          {index < activities.length - 1 && (
            <div className="absolute left-[15px] top-[42px] h-[calc(100%-18px)] w-px bg-border/50" />
          )}

          {/* Icon */}
          <div
            className={cn(
              "relative z-10 flex h-8 w-8 shrink-0 items-center justify-center rounded-full",
              getActivityIconBg(activity.type)
            )}
          >
            {getActivityIcon(activity.type)}
          </div>

          {/* Content */}
          <div className="flex-1 min-w-0">
            <div className="flex items-start justify-between gap-2">
              <div>
                <p className="text-sm">
                  <span className="font-medium text-foreground">
                    {activity.user.name}
                  </span>{" "}
                  <span className="text-muted-foreground">
                    {activity.title}
                  </span>
                </p>
                {activity.description && (
                  <p className="mt-0.5 text-xs text-muted-foreground/80 line-clamp-1">
                    {activity.description}
                  </p>
                )}
                {activity.project && (
                  <span className="mt-1 inline-flex items-center text-xs text-primary/80">
                    {activity.project}
                  </span>
                )}
              </div>
              <span className="shrink-0 text-xs text-muted-foreground/60">
                {formatDate(activity.createdAt)}
              </span>
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}
