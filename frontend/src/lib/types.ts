export type Status =
  | "success"
  | "running"
  | "pending"
  | "failed"
  | "cancelled";

export type HealthStatus = "healthy" | "degraded" | "down";

export type Environment = "production" | "staging" | "development" | "preview";

export type UserRole = "owner" | "admin" | "developer" | "viewer";

export type PipelineStepStatus =
  | "success"
  | "running"
  | "pending"
  | "failed"
  | "skipped";

export interface User {
  id: string;
  name: string;
  email: string;
  avatarUrl: string;
  role: UserRole;
  lastActive: string;
  createdAt: string;
}

export interface Project {
  id: string;
  name: string;
  description: string;
  repository: string;
  language: string;
  framework: string;
  status: Status;
  lastDeployedAt: string;
  lastDeployedBy: User;
  team: User[];
  environment: Environment;
  branch: string;
  healthStatus: HealthStatus;
  createdAt: string;
  updatedAt: string;
}

export interface PipelineStep {
  id: string;
  name: string;
  status: PipelineStepStatus;
  duration: number;
  startedAt: string;
  finishedAt: string | null;
  logs?: string;
}

export interface PipelineRun {
  id: string;
  projectId: string;
  projectName: string;
  status: Status;
  branch: string;
  commit: string;
  commitMessage: string;
  triggeredBy: User;
  steps: PipelineStep[];
  duration: number;
  startedAt: string;
  finishedAt: string | null;
  createdAt: string;
}

export interface Deployment {
  id: string;
  projectId: string;
  projectName: string;
  environment: Environment;
  status: Status;
  version: string;
  commit: string;
  commitMessage: string;
  branch: string;
  deployedBy: User;
  duration: number;
  url: string;
  rollbackAvailable: boolean;
  startedAt: string;
  finishedAt: string | null;
  createdAt: string;
}

export interface HealthCheck {
  id: string;
  name: string;
  status: HealthStatus;
  url: string;
  responseTime: number;
  uptime: number;
  lastCheckedAt: string;
}

export interface Alert {
  id: string;
  title: string;
  message: string;
  severity: "critical" | "warning" | "info";
  source: string;
  acknowledged: boolean;
  createdAt: string;
  acknowledgedAt: string | null;
  acknowledgedBy: User | null;
}

export interface MetricDataPoint {
  timestamp: string;
  value: number;
}

export interface Metric {
  id: string;
  name: string;
  unit: string;
  current: number;
  data: MetricDataPoint[];
}

export interface ActivityItem {
  id: string;
  type:
    | "deployment"
    | "pipeline"
    | "project"
    | "team"
    | "alert"
    | "rollback"
    | "config";
  title: string;
  description: string;
  user: User;
  project?: string;
  status?: Status;
  createdAt: string;
}

export interface TeamInvite {
  email: string;
  role: UserRole;
}

export interface ApiKey {
  id: string;
  name: string;
  prefix: string;
  createdAt: string;
  lastUsedAt: string | null;
  expiresAt: string | null;
}

export interface NotificationSettings {
  email: boolean;
  slack: boolean;
  deploySuccess: boolean;
  deployFailure: boolean;
  pipelineFailure: boolean;
  healthAlerts: boolean;
  teamChanges: boolean;
  weeklyReport: boolean;
}

export interface DashboardStats {
  totalProjects: number;
  activeDeployments: number;
  pipelineRuns: number;
  teamMembers: number;
  projectsTrend: number;
  deploymentsTrend: number;
  pipelinesTrend: number;
  membersTrend: number;
}

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  pageSize: number;
  totalPages: number;
}
