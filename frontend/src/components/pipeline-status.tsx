import {
  CheckCircle2,
  XCircle,
  Loader2,
  Clock,
  SkipForward,
  Circle,
} from "lucide-react";
import { cn, formatDuration } from "@/lib/utils";
import type { PipelineStep, PipelineStepStatus } from "@/lib/types";

interface PipelineStatusProps {
  steps: PipelineStep[];
  compact?: boolean;
}

function getStepIcon(status: PipelineStepStatus) {
  switch (status) {
    case "success":
      return <CheckCircle2 size={18} className="text-nexus-green-400" />;
    case "failed":
      return <XCircle size={18} className="text-nexus-red-400" />;
    case "running":
      return <Loader2 size={18} className="animate-spin text-nexus-blue-400" />;
    case "pending":
      return <Clock size={18} className="text-muted-foreground" />;
    case "skipped":
      return <SkipForward size={18} className="text-muted-foreground/50" />;
    default:
      return <Circle size={18} className="text-muted-foreground" />;
  }
}

function getConnectorColor(status: PipelineStepStatus): string {
  switch (status) {
    case "success":
      return "bg-nexus-green-400/40";
    case "failed":
      return "bg-nexus-red-400/40";
    case "running":
      return "bg-nexus-blue-400/40";
    default:
      return "bg-border";
  }
}

export function PipelineStatus({ steps, compact = false }: PipelineStatusProps) {
  if (compact) {
    return (
      <div className="flex items-center gap-1">
        {steps.map((step, index) => (
          <div key={step.id} className="flex items-center">
            <div
              className={cn(
                "h-2 w-2 rounded-full",
                step.status === "success" && "bg-nexus-green-400",
                step.status === "failed" && "bg-nexus-red-400",
                step.status === "running" &&
                  "bg-nexus-blue-400 animate-pulse-dot",
                step.status === "pending" && "bg-muted-foreground/30",
                step.status === "skipped" && "bg-muted-foreground/20"
              )}
              title={`${step.name}: ${step.status}`}
            />
            {index < steps.length - 1 && (
              <div
                className={cn(
                  "h-px w-3",
                  getConnectorColor(step.status)
                )}
              />
            )}
          </div>
        ))}
      </div>
    );
  }

  return (
    <div className="flex items-start gap-0">
      {steps.map((step, index) => (
        <div key={step.id} className="flex items-start">
          {/* Step */}
          <div className="flex flex-col items-center">
            <div
              className={cn(
                "flex h-10 w-10 items-center justify-center rounded-full border-2",
                step.status === "success" &&
                  "border-nexus-green-400/30 bg-nexus-green-400/10",
                step.status === "failed" &&
                  "border-nexus-red-400/30 bg-nexus-red-400/10",
                step.status === "running" &&
                  "border-nexus-blue-400/30 bg-nexus-blue-400/10",
                step.status === "pending" && "border-border bg-muted",
                step.status === "skipped" && "border-border/50 bg-muted/50"
              )}
            >
              {getStepIcon(step.status)}
            </div>
            <div className="mt-2 text-center">
              <p
                className={cn(
                  "text-xs font-medium",
                  step.status === "skipped"
                    ? "text-muted-foreground/50"
                    : "text-foreground"
                )}
              >
                {step.name}
              </p>
              {step.duration > 0 && (
                <p className="text-[10px] text-muted-foreground">
                  {formatDuration(step.duration)}
                </p>
              )}
            </div>
          </div>

          {/* Connector line */}
          {index < steps.length - 1 && (
            <div className="mt-5 flex items-center px-1">
              <div
                className={cn(
                  "h-0.5 w-8 md:w-12",
                  getConnectorColor(step.status)
                )}
              />
            </div>
          )}
        </div>
      ))}
    </div>
  );
}
