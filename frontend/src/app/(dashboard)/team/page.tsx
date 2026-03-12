"use client";

import { useState } from "react";
import {
  Search,
  Plus,
  MoreHorizontal,
  Mail,
  Shield,
  ShieldCheck,
  ShieldAlert,
  Eye,
  UserPlus,
  X,
  Clock,
  CheckCircle2,
  XCircle,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import type { User, UserRole } from "@/lib/types";

// Mock team members
const mockTeamMembers: (User & { status: "active" | "invited" | "inactive" })[] = [
  {
    id: "u1",
    name: "John Doe",
    email: "john@nexusops.dev",
    avatarUrl: "",
    role: "owner",
    lastActive: new Date(Date.now() - 1000 * 60 * 5).toISOString(),
    createdAt: "2024-01-01T00:00:00Z",
    status: "active",
  },
  {
    id: "u2",
    name: "Sarah Chen",
    email: "sarah@nexusops.dev",
    avatarUrl: "",
    role: "admin",
    lastActive: new Date(Date.now() - 1000 * 60 * 30).toISOString(),
    createdAt: "2024-01-15T00:00:00Z",
    status: "active",
  },
  {
    id: "u3",
    name: "Mike Wilson",
    email: "mike@nexusops.dev",
    avatarUrl: "",
    role: "developer",
    lastActive: new Date(Date.now() - 1000 * 60 * 60 * 2).toISOString(),
    createdAt: "2024-02-01T00:00:00Z",
    status: "active",
  },
  {
    id: "u4",
    name: "Emily Zhang",
    email: "emily@nexusops.dev",
    avatarUrl: "",
    role: "developer",
    lastActive: new Date(Date.now() - 1000 * 60 * 60 * 24).toISOString(),
    createdAt: "2024-02-15T00:00:00Z",
    status: "active",
  },
  {
    id: "u5",
    name: "Alex Rivera",
    email: "alex@nexusops.dev",
    avatarUrl: "",
    role: "viewer",
    lastActive: new Date(Date.now() - 1000 * 60 * 60 * 48).toISOString(),
    createdAt: "2024-03-01T00:00:00Z",
    status: "active",
  },
  {
    id: "u6",
    name: "",
    email: "newdev@nexusops.dev",
    avatarUrl: "",
    role: "developer",
    lastActive: "",
    createdAt: new Date(Date.now() - 1000 * 60 * 60 * 12).toISOString(),
    status: "invited",
  },
];

function getRoleIcon(role: UserRole) {
  switch (role) {
    case "owner":
      return <ShieldAlert className="h-3.5 w-3.5" />;
    case "admin":
      return <ShieldCheck className="h-3.5 w-3.5" />;
    case "developer":
      return <Shield className="h-3.5 w-3.5" />;
    case "viewer":
      return <Eye className="h-3.5 w-3.5" />;
  }
}

function getRoleBadgeVariant(role: UserRole) {
  switch (role) {
    case "owner":
      return "error" as const;
    case "admin":
      return "warning" as const;
    case "developer":
      return "info" as const;
    case "viewer":
      return "secondary" as const;
  }
}

function getStatusColor(status: "active" | "invited" | "inactive") {
  switch (status) {
    case "active":
      return "bg-nexus-green-400";
    case "invited":
      return "bg-yellow-400";
    case "inactive":
      return "bg-muted-foreground/40";
  }
}

function formatLastActive(dateStr: string): string {
  if (!dateStr) return "Never";
  const date = new Date(dateStr);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMins / 60);
  const diffDays = Math.floor(diffHours / 24);

  if (diffMins < 1) return "Just now";
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffHours < 24) return `${diffHours}h ago`;
  return `${diffDays}d ago`;
}

function getInitials(name: string, email: string): string {
  if (name) {
    return name
      .split(" ")
      .map((n) => n[0])
      .join("")
      .toUpperCase()
      .slice(0, 2);
  }
  return email[0].toUpperCase();
}

export default function TeamPage() {
  const [search, setSearch] = useState("");
  const [showInviteForm, setShowInviteForm] = useState(false);
  const [inviteEmail, setInviteEmail] = useState("");
  const [inviteRole, setInviteRole] = useState<UserRole>("developer");
  const [menuOpen, setMenuOpen] = useState<string | null>(null);

  const filteredMembers = mockTeamMembers.filter(
    (m) =>
      m.name.toLowerCase().includes(search.toLowerCase()) ||
      m.email.toLowerCase().includes(search.toLowerCase()) ||
      m.role.toLowerCase().includes(search.toLowerCase())
  );

  const activeCount = mockTeamMembers.filter((m) => m.status === "active").length;
  const invitedCount = mockTeamMembers.filter((m) => m.status === "invited").length;

  const handleInvite = () => {
    // Mock invite action
    setInviteEmail("");
    setShowInviteForm(false);
  };

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Team</h1>
          <p className="text-sm text-muted-foreground">
            Manage team members, roles, and permissions.
          </p>
        </div>
        <Button size="sm" onClick={() => setShowInviteForm(true)}>
          <UserPlus className="mr-2 h-3.5 w-3.5" />
          Invite Member
        </Button>
      </div>

      {/* Stats */}
      <div className="grid gap-4 sm:grid-cols-3">
        <Card>
          <CardContent className="pt-6">
            <div className="flex items-center gap-3">
              <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-nexus-green-500/10 text-nexus-green-400">
                <CheckCircle2 className="h-5 w-5" />
              </div>
              <div>
                <p className="text-2xl font-bold">{activeCount}</p>
                <p className="text-xs text-muted-foreground">Active Members</p>
              </div>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-6">
            <div className="flex items-center gap-3">
              <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-yellow-500/10 text-yellow-400">
                <Clock className="h-5 w-5" />
              </div>
              <div>
                <p className="text-2xl font-bold">{invitedCount}</p>
                <p className="text-xs text-muted-foreground">Pending Invites</p>
              </div>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-6">
            <div className="flex items-center gap-3">
              <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-nexus-blue-500/10 text-nexus-blue-400">
                <Shield className="h-5 w-5" />
              </div>
              <div>
                <p className="text-2xl font-bold">4</p>
                <p className="text-xs text-muted-foreground">Roles Configured</p>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Invite Form */}
      {showInviteForm && (
        <Card className="border-primary/30 bg-primary/5">
          <CardHeader className="flex flex-row items-center justify-between pb-4">
            <CardTitle className="text-base">Invite Team Member</CardTitle>
            <Button
              variant="ghost"
              size="icon-sm"
              onClick={() => setShowInviteForm(false)}
            >
              <X className="h-4 w-4" />
            </Button>
          </CardHeader>
          <CardContent>
            <div className="flex items-end gap-3">
              <div className="flex-1">
                <label className="mb-1.5 block text-xs font-medium text-muted-foreground">
                  Email Address
                </label>
                <Input
                  placeholder="colleague@company.com"
                  icon={<Mail size={16} />}
                  value={inviteEmail}
                  onChange={(e) => setInviteEmail(e.target.value)}
                />
              </div>
              <div className="w-40">
                <label className="mb-1.5 block text-xs font-medium text-muted-foreground">
                  Role
                </label>
                <select
                  value={inviteRole}
                  onChange={(e) => setInviteRole(e.target.value as UserRole)}
                  className="flex h-9 w-full rounded-lg border border-border bg-card px-3 text-sm outline-none transition-colors focus:border-primary focus:ring-1 focus:ring-primary"
                >
                  <option value="admin">Admin</option>
                  <option value="developer">Developer</option>
                  <option value="viewer">Viewer</option>
                </select>
              </div>
              <Button size="sm" onClick={handleInvite} disabled={!inviteEmail}>
                <Plus className="mr-2 h-3.5 w-3.5" />
                Send Invite
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Search */}
      <div className="flex-1 max-w-md">
        <Input
          placeholder="Search members..."
          icon={<Search size={16} />}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
      </div>

      {/* Members List */}
      <Card>
        <CardContent className="p-0">
          <div className="divide-y divide-border/50">
            {filteredMembers.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-16">
                <Search size={40} className="mb-3 text-muted-foreground/30" />
                <p className="text-sm text-muted-foreground">No members found</p>
              </div>
            ) : (
              filteredMembers.map((member) => (
                <div
                  key={member.id}
                  className="flex items-center justify-between px-6 py-4 transition-colors hover:bg-accent/30"
                >
                  <div className="flex items-center gap-4">
                    {/* Avatar */}
                    <div className="relative">
                      <div className="flex h-10 w-10 items-center justify-center rounded-full bg-muted text-sm font-medium">
                        {getInitials(member.name, member.email)}
                      </div>
                      <div
                        className={`absolute -bottom-0.5 -right-0.5 h-3 w-3 rounded-full border-2 border-card ${getStatusColor(member.status)}`}
                      />
                    </div>

                    {/* Info */}
                    <div>
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-medium">
                          {member.name || member.email}
                        </span>
                        {member.status === "invited" && (
                          <Badge variant="warning" className="text-[10px]">
                            Pending
                          </Badge>
                        )}
                      </div>
                      <p className="text-xs text-muted-foreground">{member.email}</p>
                    </div>
                  </div>

                  <div className="flex items-center gap-4">
                    {/* Role Badge */}
                    <Badge variant={getRoleBadgeVariant(member.role)}>
                      <span className="mr-1.5">{getRoleIcon(member.role)}</span>
                      {member.role}
                    </Badge>

                    {/* Last Active */}
                    <div className="w-20 text-right">
                      <p className="text-xs text-muted-foreground">
                        {member.status === "invited"
                          ? "Invited"
                          : formatLastActive(member.lastActive)}
                      </p>
                    </div>

                    {/* Actions */}
                    <div className="relative">
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        onClick={() =>
                          setMenuOpen(menuOpen === member.id ? null : member.id)
                        }
                        disabled={member.role === "owner"}
                      >
                        <MoreHorizontal className="h-4 w-4" />
                      </Button>
                      {menuOpen === member.id && (
                        <div className="absolute right-0 top-full z-10 mt-1 w-48 rounded-lg border border-border bg-card py-1 shadow-lg">
                          <button
                            className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-accent/50"
                            onClick={() => setMenuOpen(null)}
                          >
                            <Shield className="h-3.5 w-3.5" />
                            Change Role
                          </button>
                          {member.status === "invited" && (
                            <button
                              className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-accent/50"
                              onClick={() => setMenuOpen(null)}
                            >
                              <Mail className="h-3.5 w-3.5" />
                              Resend Invite
                            </button>
                          )}
                          <button
                            className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-nexus-red-400 hover:bg-accent/50"
                            onClick={() => setMenuOpen(null)}
                          >
                            <XCircle className="h-3.5 w-3.5" />
                            {member.status === "invited"
                              ? "Revoke Invite"
                              : "Remove Member"}
                          </button>
                        </div>
                      )}
                    </div>
                  </div>
                </div>
              ))
            )}
          </div>
        </CardContent>
      </Card>

      {/* Role Descriptions */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Role Permissions</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            {[
              {
                role: "Owner",
                icon: <ShieldAlert className="h-4 w-4" />,
                color: "text-nexus-red-400 bg-nexus-red-400/10",
                permissions: [
                  "Full account access",
                  "Billing management",
                  "Delete organization",
                  "Transfer ownership",
                ],
              },
              {
                role: "Admin",
                icon: <ShieldCheck className="h-4 w-4" />,
                color: "text-yellow-400 bg-yellow-400/10",
                permissions: [
                  "Manage team members",
                  "Configure settings",
                  "Manage API keys",
                  "All project access",
                ],
              },
              {
                role: "Developer",
                icon: <Shield className="h-4 w-4" />,
                color: "text-nexus-blue-400 bg-nexus-blue-400/10",
                permissions: [
                  "Create projects",
                  "Deploy & rollback",
                  "Run pipelines",
                  "View monitoring",
                ],
              },
              {
                role: "Viewer",
                icon: <Eye className="h-4 w-4" />,
                color: "text-muted-foreground bg-muted",
                permissions: [
                  "View projects",
                  "View deployments",
                  "View pipelines",
                  "View monitoring",
                ],
              },
            ].map((item) => (
              <div
                key={item.role}
                className="rounded-lg border border-border p-4 space-y-3"
              >
                <div className="flex items-center gap-2">
                  <div
                    className={`flex h-8 w-8 items-center justify-center rounded-lg ${item.color}`}
                  >
                    {item.icon}
                  </div>
                  <span className="text-sm font-medium">{item.role}</span>
                </div>
                <ul className="space-y-1.5">
                  {item.permissions.map((perm) => (
                    <li
                      key={perm}
                      className="flex items-center gap-2 text-xs text-muted-foreground"
                    >
                      <div className="h-1 w-1 rounded-full bg-muted-foreground/50" />
                      {perm}
                    </li>
                  ))}
                </ul>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
