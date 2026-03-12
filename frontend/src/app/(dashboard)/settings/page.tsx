"use client";

import { useState } from "react";
import {
  User,
  Key,
  Bell,
  Copy,
  Eye,
  EyeOff,
  Plus,
  Trash2,
  Check,
  Save,
  Globe,
  Mail,
  MessageSquare,
  AlertTriangle,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import type { ApiKey, NotificationSettings } from "@/lib/types";

// Mock profile data
const mockProfile = {
  name: "John Doe",
  email: "john@nexusops.dev",
  role: "owner" as const,
  avatarUrl: "",
  timezone: "America/New_York",
  createdAt: "2024-01-01T00:00:00Z",
};

// Mock API keys
const mockApiKeys: ApiKey[] = [
  {
    id: "key-1",
    name: "Production CI/CD",
    prefix: "nxo_prod_",
    createdAt: "2024-01-15T00:00:00Z",
    lastUsedAt: new Date(Date.now() - 1000 * 60 * 30).toISOString(),
    expiresAt: "2025-01-15T00:00:00Z",
  },
  {
    id: "key-2",
    name: "Staging Pipeline",
    prefix: "nxo_stg_",
    createdAt: "2024-02-01T00:00:00Z",
    lastUsedAt: new Date(Date.now() - 1000 * 60 * 60 * 24 * 3).toISOString(),
    expiresAt: "2025-02-01T00:00:00Z",
  },
  {
    id: "key-3",
    name: "Local Development",
    prefix: "nxo_dev_",
    createdAt: "2024-03-01T00:00:00Z",
    lastUsedAt: null,
    expiresAt: null,
  },
];

// Mock notification settings
const mockNotifications: NotificationSettings = {
  email: true,
  slack: true,
  deploySuccess: false,
  deployFailure: true,
  pipelineFailure: true,
  healthAlerts: true,
  teamChanges: true,
  weeklyReport: true,
};

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

export default function SettingsPage() {
  const [profile, setProfile] = useState(mockProfile);
  const [apiKeys] = useState(mockApiKeys);
  const [notifications, setNotifications] = useState(mockNotifications);
  const [showNewKeyForm, setShowNewKeyForm] = useState(false);
  const [newKeyName, setNewKeyName] = useState("");
  const [copiedKey, setCopiedKey] = useState<string | null>(null);
  const [savedProfile, setSavedProfile] = useState(false);
  const [savedNotifications, setSavedNotifications] = useState(false);

  const handleCopyKey = (prefix: string) => {
    navigator.clipboard.writeText(`${prefix}${"*".repeat(32)}`);
    setCopiedKey(prefix);
    setTimeout(() => setCopiedKey(null), 2000);
  };

  const handleSaveProfile = () => {
    setSavedProfile(true);
    setTimeout(() => setSavedProfile(false), 2000);
  };

  const handleSaveNotifications = () => {
    setSavedNotifications(true);
    setTimeout(() => setSavedNotifications(false), 2000);
  };

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Settings</h1>
        <p className="text-sm text-muted-foreground">
          Manage your account, API keys, and notification preferences.
        </p>
      </div>

      <Tabs defaultValue="profile" className="space-y-6">
        <TabsList>
          <TabsTrigger value="profile" className="gap-2">
            <User className="h-3.5 w-3.5" />
            Profile
          </TabsTrigger>
          <TabsTrigger value="api-keys" className="gap-2">
            <Key className="h-3.5 w-3.5" />
            API Keys
          </TabsTrigger>
          <TabsTrigger value="notifications" className="gap-2">
            <Bell className="h-3.5 w-3.5" />
            Notifications
          </TabsTrigger>
        </TabsList>

        {/* Profile Tab */}
        <TabsContent value="profile" className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Profile Information</CardTitle>
              <CardDescription>
                Update your personal information and preferences.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              {/* Avatar */}
              <div className="flex items-center gap-4">
                <div className="flex h-16 w-16 items-center justify-center rounded-full bg-muted text-lg font-semibold">
                  {profile.name
                    .split(" ")
                    .map((n) => n[0])
                    .join("")}
                </div>
                <div>
                  <Button variant="outline" size="sm">
                    Change Avatar
                  </Button>
                  <p className="mt-1 text-xs text-muted-foreground">
                    JPG, PNG or GIF. 1MB max.
                  </p>
                </div>
              </div>

              {/* Form Fields */}
              <div className="grid gap-4 sm:grid-cols-2">
                <div>
                  <label className="mb-1.5 block text-sm font-medium">
                    Full Name
                  </label>
                  <Input
                    value={profile.name}
                    onChange={(e) =>
                      setProfile({ ...profile, name: e.target.value })
                    }
                  />
                </div>
                <div>
                  <label className="mb-1.5 block text-sm font-medium">
                    Email Address
                  </label>
                  <Input
                    value={profile.email}
                    onChange={(e) =>
                      setProfile({ ...profile, email: e.target.value })
                    }
                    icon={<Mail size={16} />}
                  />
                </div>
                <div>
                  <label className="mb-1.5 block text-sm font-medium">
                    Timezone
                  </label>
                  <select
                    value={profile.timezone}
                    onChange={(e) =>
                      setProfile({ ...profile, timezone: e.target.value })
                    }
                    className="flex h-9 w-full rounded-lg border border-border bg-card px-3 text-sm outline-none transition-colors focus:border-primary focus:ring-1 focus:ring-primary"
                  >
                    <option value="America/New_York">Eastern Time (ET)</option>
                    <option value="America/Chicago">Central Time (CT)</option>
                    <option value="America/Denver">Mountain Time (MT)</option>
                    <option value="America/Los_Angeles">Pacific Time (PT)</option>
                    <option value="Europe/London">GMT (London)</option>
                    <option value="Europe/Berlin">CET (Berlin)</option>
                    <option value="Asia/Tokyo">JST (Tokyo)</option>
                    <option value="Asia/Shanghai">CST (Shanghai)</option>
                  </select>
                </div>
                <div>
                  <label className="mb-1.5 block text-sm font-medium">Role</label>
                  <div className="flex h-9 items-center rounded-lg border border-border bg-muted/50 px-3 text-sm text-muted-foreground">
                    <Globe className="mr-2 h-3.5 w-3.5" />
                    {profile.role.charAt(0).toUpperCase() + profile.role.slice(1)}
                  </div>
                </div>
              </div>

              <div className="flex items-center gap-3 pt-2">
                <Button size="sm" onClick={handleSaveProfile}>
                  {savedProfile ? (
                    <>
                      <Check className="mr-2 h-3.5 w-3.5" />
                      Saved
                    </>
                  ) : (
                    <>
                      <Save className="mr-2 h-3.5 w-3.5" />
                      Save Changes
                    </>
                  )}
                </Button>
                <p className="text-xs text-muted-foreground">
                  Member since {formatDate(profile.createdAt)}
                </p>
              </div>
            </CardContent>
          </Card>

          {/* Danger Zone */}
          <Card className="border-nexus-red-400/30">
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base text-nexus-red-400">
                <AlertTriangle className="h-4 w-4" />
                Danger Zone
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex items-center justify-between rounded-lg border border-border p-4">
                <div>
                  <p className="text-sm font-medium">Delete Account</p>
                  <p className="text-xs text-muted-foreground">
                    Permanently delete your account and all associated data.
                  </p>
                </div>
                <Button variant="destructive" size="sm">
                  Delete Account
                </Button>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        {/* API Keys Tab */}
        <TabsContent value="api-keys" className="space-y-6">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <div>
                <CardTitle className="text-base">API Keys</CardTitle>
                <CardDescription>
                  Manage API keys for programmatic access to NexusOps.
                </CardDescription>
              </div>
              <Button size="sm" onClick={() => setShowNewKeyForm(true)}>
                <Plus className="mr-2 h-3.5 w-3.5" />
                Create Key
              </Button>
            </CardHeader>
            <CardContent className="space-y-4">
              {/* New Key Form */}
              {showNewKeyForm && (
                <div className="flex items-end gap-3 rounded-lg border border-primary/30 bg-primary/5 p-4">
                  <div className="flex-1">
                    <label className="mb-1.5 block text-xs font-medium text-muted-foreground">
                      Key Name
                    </label>
                    <Input
                      placeholder="e.g. Production CI/CD"
                      value={newKeyName}
                      onChange={(e) => setNewKeyName(e.target.value)}
                    />
                  </div>
                  <Button
                    size="sm"
                    onClick={() => {
                      setNewKeyName("");
                      setShowNewKeyForm(false);
                    }}
                    disabled={!newKeyName}
                  >
                    Generate
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => setShowNewKeyForm(false)}
                  >
                    Cancel
                  </Button>
                </div>
              )}

              {/* Key List */}
              <div className="divide-y divide-border/50 rounded-lg border border-border">
                {apiKeys.map((key) => (
                  <div
                    key={key.id}
                    className="flex items-center justify-between px-4 py-3"
                  >
                    <div className="flex items-center gap-3">
                      <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-muted">
                        <Key className="h-4 w-4 text-muted-foreground" />
                      </div>
                      <div>
                        <p className="text-sm font-medium">{key.name}</p>
                        <div className="flex items-center gap-2 text-xs text-muted-foreground">
                          <code className="rounded bg-muted px-1.5 py-0.5 font-mono">
                            {key.prefix}{"*".repeat(16)}
                          </code>
                          <span>Created {formatDate(key.createdAt)}</span>
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center gap-3">
                      <div className="text-right text-xs text-muted-foreground">
                        {key.lastUsedAt ? (
                          <span>Last used {formatDate(key.lastUsedAt)}</span>
                        ) : (
                          <span className="text-yellow-400">Never used</span>
                        )}
                        {key.expiresAt && (
                          <p>Expires {formatDate(key.expiresAt)}</p>
                        )}
                      </div>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        onClick={() => handleCopyKey(key.prefix)}
                      >
                        {copiedKey === key.prefix ? (
                          <Check className="h-3.5 w-3.5 text-nexus-green-400" />
                        ) : (
                          <Copy className="h-3.5 w-3.5" />
                        )}
                      </Button>
                      <Button variant="ghost" size="icon-sm">
                        <Trash2 className="h-3.5 w-3.5 text-nexus-red-400" />
                      </Button>
                    </div>
                  </div>
                ))}
              </div>

              <p className="text-xs text-muted-foreground">
                API keys grant full access to your account. Keep them secure and
                never share them publicly.
              </p>
            </CardContent>
          </Card>
        </TabsContent>

        {/* Notifications Tab */}
        <TabsContent value="notifications" className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Notification Channels</CardTitle>
              <CardDescription>
                Choose how you want to receive notifications.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex items-center justify-between rounded-lg border border-border p-4">
                <div className="flex items-center gap-3">
                  <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-nexus-blue-500/10 text-nexus-blue-400">
                    <Mail className="h-4 w-4" />
                  </div>
                  <div>
                    <p className="text-sm font-medium">Email Notifications</p>
                    <p className="text-xs text-muted-foreground">
                      Receive notifications via email
                    </p>
                  </div>
                </div>
                <button
                  onClick={() =>
                    setNotifications({ ...notifications, email: !notifications.email })
                  }
                  className={`relative h-6 w-11 rounded-full transition-colors ${
                    notifications.email ? "bg-primary" : "bg-muted"
                  }`}
                >
                  <span
                    className={`absolute top-0.5 left-0.5 h-5 w-5 rounded-full bg-white transition-transform ${
                      notifications.email ? "translate-x-5" : "translate-x-0"
                    }`}
                  />
                </button>
              </div>

              <div className="flex items-center justify-between rounded-lg border border-border p-4">
                <div className="flex items-center gap-3">
                  <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-purple-500/10 text-purple-400">
                    <MessageSquare className="h-4 w-4" />
                  </div>
                  <div>
                    <p className="text-sm font-medium">Slack Notifications</p>
                    <p className="text-xs text-muted-foreground">
                      Get alerts in your Slack workspace
                    </p>
                  </div>
                </div>
                <button
                  onClick={() =>
                    setNotifications({ ...notifications, slack: !notifications.slack })
                  }
                  className={`relative h-6 w-11 rounded-full transition-colors ${
                    notifications.slack ? "bg-primary" : "bg-muted"
                  }`}
                >
                  <span
                    className={`absolute top-0.5 left-0.5 h-5 w-5 rounded-full bg-white transition-transform ${
                      notifications.slack ? "translate-x-5" : "translate-x-0"
                    }`}
                  />
                </button>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">Notification Events</CardTitle>
              <CardDescription>
                Select which events trigger notifications.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-1">
              {[
                {
                  key: "deploySuccess" as const,
                  label: "Deployment Succeeded",
                  desc: "When a deployment completes successfully",
                },
                {
                  key: "deployFailure" as const,
                  label: "Deployment Failed",
                  desc: "When a deployment fails or is rolled back",
                },
                {
                  key: "pipelineFailure" as const,
                  label: "Pipeline Failed",
                  desc: "When a CI/CD pipeline run fails",
                },
                {
                  key: "healthAlerts" as const,
                  label: "Health Alerts",
                  desc: "When a service health check fails",
                },
                {
                  key: "teamChanges" as const,
                  label: "Team Changes",
                  desc: "When team members are added or removed",
                },
                {
                  key: "weeklyReport" as const,
                  label: "Weekly Report",
                  desc: "Weekly summary of platform activity",
                },
              ].map((item) => (
                <div
                  key={item.key}
                  className="flex items-center justify-between rounded-lg px-4 py-3 transition-colors hover:bg-accent/30"
                >
                  <div>
                    <p className="text-sm font-medium">{item.label}</p>
                    <p className="text-xs text-muted-foreground">{item.desc}</p>
                  </div>
                  <button
                    onClick={() =>
                      setNotifications({
                        ...notifications,
                        [item.key]: !notifications[item.key],
                      })
                    }
                    className={`relative h-6 w-11 shrink-0 rounded-full transition-colors ${
                      notifications[item.key] ? "bg-primary" : "bg-muted"
                    }`}
                  >
                    <span
                      className={`absolute top-0.5 left-0.5 h-5 w-5 rounded-full bg-white transition-transform ${
                        notifications[item.key]
                          ? "translate-x-5"
                          : "translate-x-0"
                      }`}
                    />
                  </button>
                </div>
              ))}

              <div className="pt-4">
                <Button size="sm" onClick={handleSaveNotifications}>
                  {savedNotifications ? (
                    <>
                      <Check className="mr-2 h-3.5 w-3.5" />
                      Saved
                    </>
                  ) : (
                    <>
                      <Save className="mr-2 h-3.5 w-3.5" />
                      Save Preferences
                    </>
                  )}
                </Button>
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
