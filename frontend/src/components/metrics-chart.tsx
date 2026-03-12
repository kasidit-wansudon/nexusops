"use client";

import { BarChart3, TrendingUp, TrendingDown, Minus } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

interface MetricsChartProps {
  title: string;
  value: string;
  change?: number;
  unit?: string;
  data?: number[];
  color?: "green" | "blue" | "red" | "yellow";
}

function getTrendIcon(change: number | undefined) {
  if (change === undefined) return null;
  if (change > 0) return <TrendingUp size={14} className="text-nexus-green-400" />;
  if (change < 0) return <TrendingDown size={14} className="text-nexus-red-400" />;
  return <Minus size={14} className="text-muted-foreground" />;
}

function getBarColor(color: string): string {
  switch (color) {
    case "green":
      return "bg-nexus-green-400";
    case "blue":
      return "bg-nexus-blue-400";
    case "red":
      return "bg-nexus-red-400";
    case "yellow":
      return "bg-yellow-400";
    default:
      return "bg-primary";
  }
}

export function MetricsChart({
  title,
  value,
  change,
  unit,
  data,
  color = "green",
}: MetricsChartProps) {
  const chartData = data || [40, 65, 45, 70, 55, 80, 60, 75, 50, 85, 70, 90];
  const maxValue = Math.max(...chartData);

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground">
          {title}
        </CardTitle>
        <BarChart3 size={16} className="text-muted-foreground" />
      </CardHeader>
      <CardContent>
        <div className="flex items-baseline gap-2">
          <span className="text-2xl font-bold">{value}</span>
          {unit && (
            <span className="text-sm text-muted-foreground">{unit}</span>
          )}
        </div>

        {change !== undefined && (
          <div className="mt-1 flex items-center gap-1">
            {getTrendIcon(change)}
            <span
              className={cn(
                "text-xs font-medium",
                change > 0
                  ? "text-nexus-green-400"
                  : change < 0
                  ? "text-nexus-red-400"
                  : "text-muted-foreground"
              )}
            >
              {change > 0 ? "+" : ""}
              {change}%
            </span>
            <span className="text-xs text-muted-foreground">vs last period</span>
          </div>
        )}

        {/* Mini bar chart */}
        <div className="mt-4 flex items-end gap-1" style={{ height: 48 }}>
          {chartData.map((val, i) => (
            <div
              key={i}
              className={cn(
                "flex-1 rounded-t-sm transition-all duration-300",
                getBarColor(color),
                i === chartData.length - 1 ? "opacity-100" : "opacity-40"
              )}
              style={{
                height: `${(val / maxValue) * 100}%`,
                minHeight: 2,
              }}
            />
          ))}
        </div>
      </CardContent>
    </Card>
  );
}
