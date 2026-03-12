import type {
  Project,
  PipelineRun,
  Deployment,
  User,
  DashboardStats,
  ActivityItem,
  HealthCheck,
  Alert,
  Metric,
  ApiKey,
  NotificationSettings,
  PaginatedResponse,
} from "./types";

const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";

class ApiError extends Error {
  status: number;
  body: unknown;

  constructor(message: string, status: number, body?: unknown) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.body = body;
  }
}

function getAuthHeaders(): Record<string, string> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };

  if (typeof window !== "undefined") {
    const token = localStorage.getItem("nexusops_token");
    if (token) {
      headers["Authorization"] = `Bearer ${token}`;
    }
  }

  return headers;
}

async function fetchApi<T>(
  endpoint: string,
  options: RequestInit = {}
): Promise<T> {
  const url = `${API_BASE_URL}${endpoint}`;
  const headers = {
    ...getAuthHeaders(),
    ...((options.headers as Record<string, string>) || {}),
  };

  const response = await fetch(url, {
    ...options,
    headers,
  });

  if (!response.ok) {
    let body: unknown;
    try {
      body = await response.json();
    } catch {
      body = await response.text();
    }

    if (response.status === 401) {
      if (typeof window !== "undefined") {
        localStorage.removeItem("nexusops_token");
        window.location.href = "/login";
      }
    }

    throw new ApiError(
      `API request failed: ${response.status} ${response.statusText}`,
      response.status,
      body
    );
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return response.json();
}

// Dashboard
export async function getDashboardStats(): Promise<DashboardStats> {
  return fetchApi<DashboardStats>("/dashboard/stats");
}

export async function getRecentActivity(
  limit = 10
): Promise<ActivityItem[]> {
  return fetchApi<ActivityItem[]>(`/dashboard/activity?limit=${limit}`);
}

// Projects
export async function getProjects(
  page = 1,
  search?: string
): Promise<PaginatedResponse<Project>> {
  const params = new URLSearchParams({ page: String(page), pageSize: "12" });
  if (search) params.set("search", search);
  return fetchApi<PaginatedResponse<Project>>(`/projects?${params}`);
}

export async function getProject(id: string): Promise<Project> {
  return fetchApi<Project>(`/projects/${id}`);
}

export async function createProject(
  data: Partial<Project>
): Promise<Project> {
  return fetchApi<Project>("/projects", {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function updateProject(
  id: string,
  data: Partial<Project>
): Promise<Project> {
  return fetchApi<Project>(`/projects/${id}`, {
    method: "PATCH",
    body: JSON.stringify(data),
  });
}

export async function deleteProject(id: string): Promise<void> {
  return fetchApi<void>(`/projects/${id}`, { method: "DELETE" });
}

// Pipelines
export async function getPipelineRuns(
  page = 1,
  projectId?: string
): Promise<PaginatedResponse<PipelineRun>> {
  const params = new URLSearchParams({ page: String(page), pageSize: "20" });
  if (projectId) params.set("projectId", projectId);
  return fetchApi<PaginatedResponse<PipelineRun>>(`/pipelines?${params}`);
}

export async function getPipelineRun(id: string): Promise<PipelineRun> {
  return fetchApi<PipelineRun>(`/pipelines/${id}`);
}

export async function retryPipeline(id: string): Promise<PipelineRun> {
  return fetchApi<PipelineRun>(`/pipelines/${id}/retry`, { method: "POST" });
}

export async function cancelPipeline(id: string): Promise<PipelineRun> {
  return fetchApi<PipelineRun>(`/pipelines/${id}/cancel`, { method: "POST" });
}

export async function getPipelineLogs(
  pipelineId: string,
  stepId: string
): Promise<{ lines: Array<{ lineNumber: number; content: string; timestamp?: string; level?: string }> }> {
  return fetchApi(`/pipelines/${pipelineId}/steps/${stepId}/logs`);
}

// Deployments
export async function getDeployments(
  page = 1,
  environment?: string
): Promise<PaginatedResponse<Deployment>> {
  const params = new URLSearchParams({ page: String(page), pageSize: "20" });
  if (environment) params.set("environment", environment);
  return fetchApi<PaginatedResponse<Deployment>>(`/deployments?${params}`);
}

export async function getDeployment(id: string): Promise<Deployment> {
  return fetchApi<Deployment>(`/deployments/${id}`);
}

export async function rollbackDeployment(
  id: string
): Promise<Deployment> {
  return fetchApi<Deployment>(`/deployments/${id}/rollback`, {
    method: "POST",
  });
}

export async function scaleDeployment(
  id: string,
  replicas: number
): Promise<Deployment> {
  return fetchApi<Deployment>(`/deployments/${id}/scale`, {
    method: "POST",
    body: JSON.stringify({ replicas }),
  });
}

export async function promoteDeployment(
  id: string,
  targetEnvironment: string
): Promise<Deployment> {
  return fetchApi<Deployment>(`/deployments/${id}/promote`, {
    method: "POST",
    body: JSON.stringify({ targetEnvironment }),
  });
}

// Monitoring
export async function getHealthChecks(): Promise<HealthCheck[]> {
  return fetchApi<HealthCheck[]>("/monitoring/health");
}

export async function getAlerts(): Promise<Alert[]> {
  return fetchApi<Alert[]>("/monitoring/alerts");
}

export async function acknowledgeAlert(id: string): Promise<Alert> {
  return fetchApi<Alert>(`/monitoring/alerts/${id}/acknowledge`, {
    method: "POST",
  });
}

export async function getMetrics(
  timeRange = "24h"
): Promise<Metric[]> {
  return fetchApi<Metric[]>(`/monitoring/metrics?range=${timeRange}`);
}

// Team
export async function getTeamMembers(): Promise<User[]> {
  return fetchApi<User[]>("/team/members");
}

export async function inviteTeamMember(
  email: string,
  role: string
): Promise<void> {
  return fetchApi<void>("/team/invite", {
    method: "POST",
    body: JSON.stringify({ email, role }),
  });
}

export async function removeTeamMember(userId: string): Promise<void> {
  return fetchApi<void>(`/team/members/${userId}`, { method: "DELETE" });
}

export async function updateMemberRole(
  userId: string,
  role: string
): Promise<User> {
  return fetchApi<User>(`/team/members/${userId}/role`, {
    method: "PATCH",
    body: JSON.stringify({ role }),
  });
}

// Settings
export async function getApiKeys(): Promise<ApiKey[]> {
  return fetchApi<ApiKey[]>("/settings/api-keys");
}

export async function createApiKey(name: string): Promise<ApiKey & { key: string }> {
  return fetchApi<ApiKey & { key: string }>("/settings/api-keys", {
    method: "POST",
    body: JSON.stringify({ name }),
  });
}

export async function deleteApiKey(id: string): Promise<void> {
  return fetchApi<void>(`/settings/api-keys/${id}`, { method: "DELETE" });
}

export async function getNotificationSettings(): Promise<NotificationSettings> {
  return fetchApi<NotificationSettings>("/settings/notifications");
}

export async function updateNotificationSettings(
  settings: Partial<NotificationSettings>
): Promise<NotificationSettings> {
  return fetchApi<NotificationSettings>("/settings/notifications", {
    method: "PATCH",
    body: JSON.stringify(settings),
  });
}

// Auth
export async function loginWithOAuth(
  provider: "github" | "gitlab"
): Promise<{ redirectUrl: string }> {
  return fetchApi<{ redirectUrl: string }>(`/auth/${provider}/login`);
}

export async function handleOAuthCallback(
  provider: string,
  code: string
): Promise<{ token: string; user: User }> {
  return fetchApi<{ token: string; user: User }>(`/auth/${provider}/callback`, {
    method: "POST",
    body: JSON.stringify({ code }),
  });
}

export async function getCurrentUser(): Promise<User> {
  return fetchApi<User>("/auth/me");
}

export async function logout(): Promise<void> {
  if (typeof window !== "undefined") {
    localStorage.removeItem("nexusops_token");
  }
  return fetchApi<void>("/auth/logout", { method: "POST" });
}

export { ApiError };
