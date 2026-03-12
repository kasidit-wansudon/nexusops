"use client";

import React from "react";
import { TrendingUp, TrendingDown, Minus } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";

interface StatsCardProps {
  title: string;
  value: string | number;
  icon: React.ReactNode;
  trend?: {
    value: number;
    label?: string;
  };
  description?: string;
  className?: string;
}

export function StatsCard({
  title,
  value,
  icon,
  trend,
  description,
  className,
}: StatsCardProps) {
  const trendDirection =
    trend && trend.value > 0
      ? "up"
      : trend && trend.value < 0
        ? "down"
        : "flat";

  return (
    <Card className={cn("overflow-hidden", className)}>
      <CardContent className="p-6">
        <div className="flex items-start justify-between">
          <div className="flex-1 min-w-0">
            <p className="text-sm font-medium text-muted-foreground truncate">
              {title}
            </p>
            <p className="text-2xl font-bold text-foreground mt-2">{value}</p>
            {(trend || description) && (
              <div className="flex items-center gap-2 mt-2">
                {trend && (
                  <span
                    className={cn(
                      "flex items-center gap-0.5 text-xs font-medium",
                      trendDirection === "up" && "text-nexus-green-400",
                      trendDirection === "down" && "text-nexus-red-400",
                      trendDirection === "flat" && "text-muted-foreground"
                    )}
                  >
                    {trendDirection === "up" && (
                      <TrendingUp className="h-3 w-3" />
                    )}
                    {trendDirection === "down" && (
                      <TrendingDown className="h-3 w-3" />
                    )}
                    {trendDirection === "flat" && (
                      <Minus className="h-3 w-3" />
                    )}
                    {trend.value > 0 ? "+" : ""}
                    {trend.value}%
                  </span>
                )}
                {(trend?.label || description) && (
                  <span className="text-xs text-muted-foreground">
                    {trend?.label || description}
                  </span>
                )}
              </div>
            )}
          </div>
          <div className="p-2.5 rounded-lg bg-primary/10 text-primary shrink-0">
            {icon}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

export default StatsCard;
