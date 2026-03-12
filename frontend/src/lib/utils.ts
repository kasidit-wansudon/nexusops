import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function formatDate(date: string | Date): string {
  const d = new Date(date);
  const now = new Date();
  const diffMs = now.getTime() - d.getTime();
  const diffSec = Math.floor(diffMs / 1000);
  const diffMin = Math.floor(diffSec / 60);
  const diffHour = Math.floor(diffMin / 60);
  const diffDay = Math.floor(diffHour / 24);

  if (diffSec < 60) return "just now";
  if (diffMin < 60) return `${diffMin}m ago`;
  if (diffHour < 24) return `${diffHour}h ago`;
  if (diffDay < 7) return `${diffDay}d ago`;

  return d.toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: d.getFullYear() !== now.getFullYear() ? "numeric" : undefined,
  });
}

export function formatDuration(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  const min = Math.floor(seconds / 60);
  const sec = seconds % 60;
  if (min < 60) return `${min}m ${sec}s`;
  const hour = Math.floor(min / 60);
  const remainMin = min % 60;
  return `${hour}h ${remainMin}m`;
}

export function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
}

export type StatusType =
  | "success"
  | "running"
  | "pending"
  | "failed"
  | "cancelled"
  | "warning"
  | "healthy"
  | "degraded"
  | "down";

export function statusColor(status: StatusType): string {
  const colors: Record<StatusType, string> = {
    success: "text-nexus-green-400 bg-nexus-green-400/10 border-nexus-green-400/20",
    running: "text-nexus-blue-400 bg-nexus-blue-400/10 border-nexus-blue-400/20",
    pending: "text-yellow-400 bg-yellow-400/10 border-yellow-400/20",
    failed: "text-nexus-red-400 bg-nexus-red-400/10 border-nexus-red-400/20",
    cancelled: "text-nexus-gray-400 bg-nexus-gray-400/10 border-nexus-gray-400/20",
    warning: "text-yellow-400 bg-yellow-400/10 border-yellow-400/20",
    healthy: "text-nexus-green-400 bg-nexus-green-400/10 border-nexus-green-400/20",
    degraded: "text-yellow-400 bg-yellow-400/10 border-yellow-400/20",
    down: "text-nexus-red-400 bg-nexus-red-400/10 border-nexus-red-400/20",
  };
  return colors[status] || colors.pending;
}

export function statusDot(status: StatusType): string {
  const colors: Record<StatusType, string> = {
    success: "bg-nexus-green-400",
    running: "bg-nexus-blue-400 animate-pulse-dot",
    pending: "bg-yellow-400",
    failed: "bg-nexus-red-400",
    cancelled: "bg-nexus-gray-400",
    warning: "bg-yellow-400 animate-pulse-dot",
    healthy: "bg-nexus-green-400",
    degraded: "bg-yellow-400 animate-pulse-dot",
    down: "bg-nexus-red-400 animate-pulse-dot",
  };
  return colors[status] || colors.pending;
}

export function truncateHash(hash: string, length = 7): string {
  return hash.substring(0, length);
}

export function generateId(): string {
  return Math.random().toString(36).substring(2, 9);
}
