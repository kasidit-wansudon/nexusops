"use client";

import React from "react";
import {
  CheckCircle2,
  XCircle,
  Loader2,
  Clock,
  MinusCircle,
  ChevronRight,
} from "lucide-react";
import { cn, formatDuration, type StatusType } from "@/lib/utils";

export interface PipelineStep {
  id: string;
  name: string;
  status: StatusType;
  duration?: number;
  startedAt?: string;
  finishedAt?: string;
  logUrl?: string;
}

export interface PipelineStage {
  id: string;
  name: string;
  status: StatusType;
  steps: PipelineStep[];
}

interface StageViewerProps {
  stages: PipelineStage[];
  activeStepId?: string;
  onStepClick?: (stageId: string, stepId: string) => void;
}

function StatusIcon({
  status,
  className,
}: {
  status: StatusType;
  className?: string;
}) {
  const iconProps = { className: cn("h-4 w-4", className) };

  switch (status) {
    case "success":
      return <CheckCircle2 {...iconProps} className={cn(iconProps.className, "text-nexus-green-400")} />;
    case "failed":
      return <XCircle {...iconProps} className={cn(iconProps.className, "text-nexus-red-400")} />;
    case "running":
      return <Loader2 {...iconProps} className={cn(iconProps.className, "text-nexus-blue-400 animate-spin")} />;
    case "pending":
      return <Clock {...iconProps} className={cn(iconProps.className, "text-nexus-gray-400")} />;
    case "cancelled":
      return <MinusCircle {...iconProps} className={cn(iconProps.className, "text-nexus-gray-400")} />;
    default:
      return <Clock {...iconProps} className={cn(iconProps.className, "text-nexus-gray-400")} />;
  }
}

function StageStatusBar({ status }: { status: StatusType }) {
  return (
    <div
      className={cn(
        "h-1 w-full rounded-full",
        status === "success" && "bg-nexus-green-400",
        status === "failed" && "bg-nexus-red-400",
        status === "running" && "bg-nexus-blue-400 animate-pulse-dot",
        status === "pending" && "bg-nexus-gray-700",
        status === "cancelled" && "bg-nexus-gray-600"
      )}
    />
  );
}

export function StageViewer({
  stages,
  activeStepId,
  onStepClick,
}: StageViewerProps) {
  return (
    <div className="w-full overflow-x-auto scrollbar-thin">
      <div className="flex items-start gap-0 min-w-max p-4">
        {stages.map((stage, stageIndex) => (
          <React.Fragment key={stage.id}>
            <div className="flex flex-col min-w-[200px] max-w-[260px]">
              {/* Stage Header */}
              <div className="mb-2">
                <div className="flex items-center gap-2 mb-1">
                  <StatusIcon status={stage.status} />
                  <span className="text-sm font-medium text-foreground truncate">
                    {stage.name}
                  </span>
                </div>
                <StageStatusBar status={stage.status} />
              </div>

              {/* Steps */}
              <div className="flex flex-col gap-1">
                {stage.steps.map((step) => (
                  <button
                    key={step.id}
                    onClick={() => onStepClick?.(stage.id, step.id)}
                    className={cn(
                      "flex items-center gap-2 px-3 py-2 rounded-md text-left transition-colors",
                      "hover:bg-accent/50",
                      activeStepId === step.id &&
                        "bg-accent border border-border"
                    )}
                  >
                    <StatusIcon status={step.status} className="h-3.5 w-3.5 shrink-0" />
                    <div className="flex-1 min-w-0">
                      <p className="text-xs font-medium text-foreground truncate">
                        {step.name}
                      </p>
                      {step.duration !== undefined && (
                        <p className="text-[10px] text-muted-foreground">
                          {formatDuration(step.duration)}
                        </p>
                      )}
                    </div>
                  </button>
                ))}
              </div>
            </div>

            {/* Connector */}
            {stageIndex < stages.length - 1 && (
              <div className="flex items-center self-center mt-4 px-1">
                <div className="w-6 h-[2px] bg-border" />
                <ChevronRight className="h-4 w-4 text-muted-foreground -ml-1" />
              </div>
            )}
          </React.Fragment>
        ))}
      </div>
    </div>
  );
}

export default StageViewer;
