"use client";

import { useState } from "react";
import {
  Activity,
  CheckCircle2,
  AlertTriangle,
  XCircle,
  Clock,
  Globe,
  Wifi,
  Server,
  Database,
  Bell,
  Eye,
  RefreshCw,
} from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { MetricsChart } from "@/components/metrics-chart";
import { cn, formatDate } from "@/lib/utils";
import type { HealthCheck, Alert, HealthStatus } from "@/lib/types";

const mockHealthChecks: HealthCheck[] = [
  {
    id: "hc-1",
    name: "nexus-api",
    status: "healthy",
    url: "https://api.nexusops.dev/health",
    responseTime: 42,
    uptime: 99.98,
    lastCheckedAt: new Date(Date.now() - 1000 * 30).toISOString(),
  },
  {
    id: "hc-2",
    name: "nexus-web",
    status: "healthy",
    url: "https://app.nexusops.dev",
    responseTime: 128,
    uptime: 99.95,
    lastCheckedAt: new Date(Date.now() - 1000 * 30).toISOString(),
  },
  {
    id: "hc-3",
    name: "auth-service",
    status: "healthy",
    url: "https://auth.nexusops.dev/health",
    responseTime: 35,
    uptime: 99.99,
    lastCheckedAt: new Date(Date.now() - 1000 * 30).toISOString(),
  },
  {
    id: "hc-4",
    name: "data-pipeline",
    status: "degraded",
    url: "https://data.nexusops.dev/health",
    responseTime: 890,
    uptime: 98.5,
    lastCheckedAt: new Date(Date.now() - 1000 * 30).toISOString(),
  },
  {
    id: "hc-5",
    name: "notification-svc",
    status: "healthy",
    url: "https://notify.nexusops.dev/health",
    responseTime: 56,
    uptime: 99.97,
    lastCheckedAt: new Date(Date.now() - 1000 * 30).toISOString(),
  },
  {
    id: "hc-6",
    name: "postgres-primary",
    status: "healthy",
    url: "internal://db-primary:5432",
    responseTime: 8,
    uptime: 99.99,
    lastCheckedAt: new Date(Date.now() - 1000 * 30).toISOString(),
  },
  {
    id: "hc-7",
    name: "redis-cache",
    status: "healthy",
    url: "internal://redis:6379",
    responseTime: 2,
    uptime: 100,
    lastCheckedAt: new Date(Date.now() - 1000 * 30).toISOString(),
  },
  {
    id: "hc-8",
    name: "cdn-edge",
    status: "down",
    url: "https://cdn.nexusops.dev",
    responseTime: 0,
    uptime: 95.2,
    lastCheckedAt: new Date(Date.now() - 1000 * 60 * 5).toISOString(),
  },
];

const mockAlerts: Alert[] = [
  {
    id: "alert-1",
    title: "CDN Edge nodes unreachable",
    message: "Multiple CDN edge nodes in us-east-1 are returning 503 errors. Affecting static asset delivery.",
    severity: "critical",
    source: "cdn-edge",
    acknowledged: false,
    createdAt: new Date(Date.now() - 1000 * 60 * 5).toISOString(),
    acknowledgedAt: null,
    acknowledgedBy: null,
  },
  {
    id: "alert-2",
    title: "High response time on data-pipeline",
    message: "Average response time exceeded 500ms threshold for the past 10 minutes.",
    severity: "warning",
    source: "data-pipeline",
    acknowledged: false,
    createdAt: new Date(Date.now() - 1000 * 60 * 15).toISOString(),
    acknowledgedAt: null,
    acknowledgedBy: null,
  },
  {
    id: "alert-3",
    title: "Memory usage above 80%",
    message: "nexus-api production instance memory usage at 84%. Consider scaling up.",
    severity: "warning",
    source: "nexus-api",
    acknowledged: true,
    createdAt: new Date(Date.now() - 1000 * 60 * 60 * 2).toISOString(),
    acknowledgedAt: new Date(Date.now() - 1000 * 60 * 60).toISOString(),
    acknowledgedBy: {
      id: "u1",
      name: "John Doe",
      email: "john@nexusops.dev",
      avatarUrl: "",
      role: "admin",
      lastActive: new Date().toISOString(),
      createdAt: "2024-01-01T00:00:00Z",
    },
  },
  {
    id: "alert-4",
    title: "SSL certificate expiring",
    message: "SSL certificate for auth.nexusops.dev expires in 14 days.",
    severity: "info",
    source: "auth-service",
    acknowledged: true,
    createdAt: new Date(Date.now() - 1000 * 60 * 60 * 24).toISOString(),
    acknowledgedAt: new Date(Date.now() - 1000 * 60 * 60 * 12).toISOString(),
    acknowledgedBy: {
      id: "u2",
      name: "Sarah Chen",
      email: "sarah@nexusops.dev",
      avatarUrl: "",
      role: "developer",
      lastActive: new Date().toISOString(),
      createdAt: "2024-01-15T00:00:00Z",
    },
  },
];

function getHealthIcon(status: HealthStatus) {
  switch (status) {
    case "healthy":
      return <CheckCircle2 size={20} className="text-nexus-green-400" />;
    case "degraded":
      return <AlertTriangle size={20} className="text-yellow-400" />;
    case "down":
      return <XCircle size={20} className="text-nexus-red-400" />;
  }
}

function getHealthBg(status: HealthStatus) {
  switch (status) {
    case "healthy":
      return "border-nexus-green-400/20 bg-nexus-green-400/5";
    case "degraded":
      return "border-yellow-400/20 bg-yellow-400/5";
    case "down":
      return "border-nexus-red-400/20 bg-nexus-red-400/5";
  }
}

function getSeverityBadge(severity: Alert["severity"]) {
  switch (severity) {
    case "critical":
      return "error" as const;
    case "warning":
      return "warning" as const;
    case "info":
      return "info" as const;
  }
}

function getServiceIcon(name: string) {
  if (name.includes("postgres") || name.includes("db")) return <Database size={16} />;
  if (name.includes("redis") || name.includes("cache")) return <Server size={16} />;
  if (name.includes("cdn")) return <Globe size={16} />;
  return <Wifi size={16} />;
}

export default function MonitoringPage() {
  const [refreshing, setRefreshing] = useState(false);

  const handleRefresh = () => {
    setRefreshing(true);
    setTimeout(() => setRefreshing(false), 1000);
  };

  const healthyCount = mockHealthChecks.filter((h) => h.status === "healthy").length;
  const degradedCount = mockHealthChecks.filter((h) => h.status === "degraded").length;
  const downCount = mockHealthChecks.filter((h) => h.status === "down").length;
  const unacknowledgedAlerts = mockAlerts.filter((a) => !a.acknowledged).length;

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Monitoring</h1>
          <p className="text-sm text-muted-foreground">
            System health, metrics, and alerts overview.
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={handleRefresh}>
          <RefreshCw className={cn("mr-2 h-3.5 w-3.5", refreshing && "animate-spin")} />
          Refresh
        </Button>
      </div>

      {/* Summary Stats */}
      <div className="grid gap-4 sm:grid-cols-4">
        <Card>
          <CardContent className="flex items-center gap-3 p-4">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-nexus-green-500/10">
              <CheckCircle2 size={20} className="text-nexus-green-400" />
            </div>
            <div>
              <p className="text-2xl font-bold">{healthyCount}</p>
              <p className="text-xs text-muted-foreground">Healthy Services</p>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="flex items-center gap-3 p-4">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-yellow-500/10">
              <AlertTriangle size={20} className="text-yellow-400" />
            </div>
            <div>
              <p className="text-2xl font-bold">{degradedCount}</p>
              <p className="text-xs text-muted-foreground">Degraded</p>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="flex items-center gap-3 p-4">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-nexus-red-500/10">
              <XCircle size={20} className="text-nexus-red-400" />
            </div>
            <div>
              <p className="text-2xl font-bold">{downCount}</p>
              <p className="text-xs text-muted-foreground">Down</p>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="flex items-center gap-3 p-4">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-nexus-blue-500/10">
              <Bell size={20} className="text-nexus-blue-400" />
            </div>
            <div>
              <p className="text-2xl font-bold">{unacknowledgedAlerts}</p>
              <p className="text-xs text-muted-foreground">Active Alerts</p>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Metrics Charts */}
      <div className="grid gap-4 lg:grid-cols-3">
        <MetricsChart
          title="Avg Response Time"
          value="67ms"
          change={-12}
          unit=""
          color="green"
          data={[85, 78, 72, 80, 68, 75, 65, 70, 62, 68, 64, 67]}
        />
        <MetricsChart
          title="Request Rate"
          value="2.4k"
          change={15}
          unit="req/min"
          color="blue"
          data={[1800, 2000, 1900, 2200, 2100, 2400, 2300, 2500, 2200, 2600, 2400, 2400]}
        />
        <MetricsChart
          title="Error Rate"
          value="0.12%"
          change={-5}
          unit=""
          color="red"
          data={[0.2, 0.18, 0.15, 0.22, 0.14, 0.16, 0.13, 0.15, 0.11, 0.14, 0.12, 0.12]}
        />
      </div>

      {/* Health Status Grid & Alerts */}
      <div className="grid gap-6 lg:grid-cols-5">
        {/* Health Checks */}
        <Card className="lg:col-span-3">
          <CardHeader>
            <CardTitle className="text-base">Service Health</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid gap-3 sm:grid-cols-2">
              {mockHealthChecks.map((check) => (
                <div
                  key={check.id}
                  className={cn(
                    "flex items-center justify-between rounded-lg border p-3 transition-colors",
                    getHealthBg(check.status)
                  )}
                >
                  <div className="flex items-center gap-3">
                    <div className="text-muted-foreground">
                      {getServiceIcon(check.name)}
                    </div>
                    <div>
                      <p className="text-sm font-medium">{check.name}</p>
                      <div className="flex items-center gap-2 text-xs text-muted-foreground">
                        <span>{check.responseTime > 0 ? `${check.responseTime}ms` : "N/A"}</span>
                        <span>{check.uptime}% uptime</span>
                      </div>
                    </div>
                  </div>
                  {getHealthIcon(check.status)}
                </div>
              ))}
            </div>
          </CardContent>
        </Card>

        {/* Alerts */}
        <Card className="lg:col-span-2">
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle className="text-base">Alerts</CardTitle>
            {unacknowledgedAlerts > 0 && (
              <Badge variant="error" dot>
                {unacknowledgedAlerts} active
              </Badge>
            )}
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              {mockAlerts.map((alert) => (
                <div
                  key={alert.id}
                  className={cn(
                    "rounded-lg border p-3",
                    !alert.acknowledged
                      ? "border-border bg-accent/30"
                      : "border-border/50 opacity-60"
                  )}
                >
                  <div className="flex items-start justify-between gap-2">
                    <div className="flex-1">
                      <div className="flex items-center gap-2">
                        <Badge variant={getSeverityBadge(alert.severity)}>
                          {alert.severity}
                        </Badge>
                        <span className="text-xs text-muted-foreground">
                          {alert.source}
                        </span>
                      </div>
                      <p className="mt-1.5 text-sm font-medium">{alert.title}</p>
                      <p className="mt-0.5 text-xs text-muted-foreground line-clamp-2">
                        {alert.message}
                      </p>
                      <div className="mt-2 flex items-center gap-2 text-xs text-muted-foreground">
                        <Clock size={10} />
                        <span>{formatDate(alert.createdAt)}</span>
                        {alert.acknowledged && alert.acknowledgedBy && (
                          <span>
                            Ack by {alert.acknowledgedBy.name}
                          </span>
                        )}
                      </div>
                    </div>
                    {!alert.acknowledged && (
                      <Button variant="ghost" size="icon-sm" title="Acknowledge">
                        <Eye size={14} />
                      </Button>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
