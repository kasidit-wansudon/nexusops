"use client";

import React, { useState } from "react";
import {
  LineChart,
  Line,
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  TooltipProps,
} from "recharts";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

export interface MetricDataPoint {
  timestamp: string;
  value: number;
  label?: string;
}

export type ChartType = "line" | "area";
export type TimeRange = "1h" | "6h" | "24h" | "7d" | "30d";

interface MetricChartProps {
  title: string;
  data: MetricDataPoint[];
  type?: ChartType;
  color?: string;
  unit?: string;
  timeRange?: TimeRange;
  onTimeRangeChange?: (range: TimeRange) => void;
  height?: number;
  className?: string;
  showGrid?: boolean;
  fillOpacity?: number;
  gradientId?: string;
}

const timeRanges: { value: TimeRange; label: string }[] = [
  { value: "1h", label: "1H" },
  { value: "6h", label: "6H" },
  { value: "24h", label: "24H" },
  { value: "7d", label: "7D" },
  { value: "30d", label: "30D" },
];

function CustomTooltip({
  active,
  payload,
  unit,
}: TooltipProps<number, string> & { unit?: string }) {
  if (!active || !payload || payload.length === 0) return null;

  const data = payload[0];
  return (
    <div className="custom-tooltip">
      <p className="text-xs text-muted-foreground">{data.payload.timestamp}</p>
      <p className="text-sm font-semibold text-foreground">
        {typeof data.value === "number" ? data.value.toFixed(1) : data.value}
        {unit && <span className="text-muted-foreground ml-1">{unit}</span>}
      </p>
    </div>
  );
}

export function MetricChart({
  title,
  data,
  type = "area",
  color = "#10b981",
  unit,
  timeRange = "24h",
  onTimeRangeChange,
  height = 200,
  className,
  showGrid = true,
  fillOpacity = 0.1,
  gradientId,
}: MetricChartProps) {
  const [activeRange, setActiveRange] = useState<TimeRange>(timeRange);
  const gId = gradientId || `gradient-${title.replace(/\s/g, "-").toLowerCase()}`;

  const handleRangeChange = (range: TimeRange) => {
    setActiveRange(range);
    onTimeRangeChange?.(range);
  };

  const currentValue = data.length > 0 ? data[data.length - 1].value : 0;

  return (
    <Card className={cn("overflow-hidden", className)}>
      <CardHeader className="pb-2">
        <div className="flex items-center justify-between">
          <div>
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {title}
            </CardTitle>
            <p className="text-2xl font-bold text-foreground mt-1">
              {currentValue.toFixed(1)}
              {unit && (
                <span className="text-sm font-normal text-muted-foreground ml-1">
                  {unit}
                </span>
              )}
            </p>
          </div>
          <div className="flex items-center gap-0.5 bg-muted rounded-md p-0.5">
            {timeRanges.map((range) => (
              <button
                key={range.value}
                onClick={() => handleRangeChange(range.value)}
                className={cn(
                  "px-2 py-1 text-[10px] font-medium rounded transition-colors",
                  activeRange === range.value
                    ? "bg-background text-foreground shadow-sm"
                    : "text-muted-foreground hover:text-foreground"
                )}
              >
                {range.label}
              </button>
            ))}
          </div>
        </div>
      </CardHeader>
      <CardContent className="p-0 pb-2">
        <ResponsiveContainer width="100%" height={height}>
          {type === "area" ? (
            <AreaChart
              data={data}
              margin={{ top: 5, right: 10, left: -20, bottom: 0 }}
            >
              <defs>
                <linearGradient id={gId} x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor={color} stopOpacity={fillOpacity} />
                  <stop offset="100%" stopColor={color} stopOpacity={0} />
                </linearGradient>
              </defs>
              {showGrid && (
                <CartesianGrid
                  strokeDasharray="3 3"
                  stroke="hsl(217.2, 32.6%, 17.5%)"
                  vertical={false}
                />
              )}
              <XAxis
                dataKey="timestamp"
                axisLine={false}
                tickLine={false}
                tick={{ fontSize: 10, fill: "hsl(215, 20.2%, 55.1%)" }}
                interval="preserveStartEnd"
              />
              <YAxis
                axisLine={false}
                tickLine={false}
                tick={{ fontSize: 10, fill: "hsl(215, 20.2%, 55.1%)" }}
              />
              <Tooltip content={<CustomTooltip unit={unit} />} />
              <Area
                type="monotone"
                dataKey="value"
                stroke={color}
                strokeWidth={2}
                fill={`url(#${gId})`}
                animationDuration={500}
              />
            </AreaChart>
          ) : (
            <LineChart
              data={data}
              margin={{ top: 5, right: 10, left: -20, bottom: 0 }}
            >
              {showGrid && (
                <CartesianGrid
                  strokeDasharray="3 3"
                  stroke="hsl(217.2, 32.6%, 17.5%)"
                  vertical={false}
                />
              )}
              <XAxis
                dataKey="timestamp"
                axisLine={false}
                tickLine={false}
                tick={{ fontSize: 10, fill: "hsl(215, 20.2%, 55.1%)" }}
                interval="preserveStartEnd"
              />
              <YAxis
                axisLine={false}
                tickLine={false}
                tick={{ fontSize: 10, fill: "hsl(215, 20.2%, 55.1%)" }}
              />
              <Tooltip content={<CustomTooltip unit={unit} />} />
              <Line
                type="monotone"
                dataKey="value"
                stroke={color}
                strokeWidth={2}
                dot={false}
                animationDuration={500}
              />
            </LineChart>
          )}
        </ResponsiveContainer>
      </CardContent>
    </Card>
  );
}

export default MetricChart;
