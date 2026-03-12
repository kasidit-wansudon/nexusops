"use client";

import { useState } from "react";
import { Search, Bell, ChevronDown, LogOut, User, Settings } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export function Header() {
  const [showDropdown, setShowDropdown] = useState(false);
  const [showNotifications, setShowNotifications] = useState(false);

  const notifications = [
    {
      id: "1",
      title: "Deployment succeeded",
      message: "nexus-api deployed to production",
      time: "2m ago",
      read: false,
    },
    {
      id: "2",
      title: "Pipeline failed",
      message: "Build #847 failed on main branch",
      time: "15m ago",
      read: false,
    },
    {
      id: "3",
      title: "New team member",
      message: "Sarah Chen joined the team",
      time: "1h ago",
      read: true,
    },
  ];

  const unreadCount = notifications.filter((n) => !n.read).length;

  return (
    <header className="sticky top-0 z-40 flex h-16 items-center justify-between border-b border-border bg-card/80 px-6 backdrop-blur-sm">
      {/* Search */}
      <div className="w-full max-w-md">
        <Input
          placeholder="Search projects, pipelines, deployments..."
          icon={<Search size={16} />}
          className="bg-background/50"
        />
      </div>

      {/* Right side */}
      <div className="flex items-center gap-2">
        {/* Notifications */}
        <div className="relative">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => {
              setShowNotifications(!showNotifications);
              setShowDropdown(false);
            }}
            className="relative"
          >
            <Bell size={18} />
            {unreadCount > 0 && (
              <span className="absolute right-1.5 top-1.5 flex h-4 w-4 items-center justify-center rounded-full bg-nexus-red-500 text-[10px] font-bold text-white">
                {unreadCount}
              </span>
            )}
          </Button>

          {showNotifications && (
            <div className="absolute right-0 top-full mt-2 w-80 rounded-xl border border-border bg-card shadow-lg">
              <div className="flex items-center justify-between border-b border-border p-4">
                <h3 className="font-semibold">Notifications</h3>
                <button className="text-xs text-primary hover:underline">
                  Mark all read
                </button>
              </div>
              <div className="max-h-80 overflow-y-auto scrollbar-thin">
                {notifications.map((notification) => (
                  <div
                    key={notification.id}
                    className={cn(
                      "flex gap-3 border-b border-border/50 px-4 py-3 transition-colors hover:bg-accent/50",
                      !notification.read && "bg-primary/5"
                    )}
                  >
                    {!notification.read && (
                      <span className="mt-1.5 h-2 w-2 shrink-0 rounded-full bg-primary" />
                    )}
                    <div className={cn(!notification.read ? "" : "ml-5")}>
                      <p className="text-sm font-medium">
                        {notification.title}
                      </p>
                      <p className="text-xs text-muted-foreground">
                        {notification.message}
                      </p>
                      <p className="mt-1 text-xs text-muted-foreground/70">
                        {notification.time}
                      </p>
                    </div>
                  </div>
                ))}
              </div>
              <div className="border-t border-border p-2">
                <button className="w-full rounded-lg py-2 text-center text-sm text-primary hover:bg-accent/50">
                  View all notifications
                </button>
              </div>
            </div>
          )}
        </div>

        {/* User dropdown */}
        <div className="relative">
          <button
            onClick={() => {
              setShowDropdown(!showDropdown);
              setShowNotifications(false);
            }}
            className="flex items-center gap-2 rounded-lg px-2 py-1.5 transition-colors hover:bg-accent"
          >
            <div className="flex h-8 w-8 items-center justify-center rounded-full bg-gradient-to-br from-nexus-blue-500 to-nexus-green-500 text-sm font-bold text-white">
              JD
            </div>
            <div className="hidden text-left md:block">
              <p className="text-sm font-medium">John Doe</p>
              <p className="text-xs text-muted-foreground">Admin</p>
            </div>
            <ChevronDown size={14} className="text-muted-foreground" />
          </button>

          {showDropdown && (
            <div className="absolute right-0 top-full mt-2 w-48 rounded-xl border border-border bg-card py-1 shadow-lg">
              <a
                href="/settings"
                className="flex items-center gap-2 px-4 py-2 text-sm text-foreground transition-colors hover:bg-accent"
              >
                <User size={14} />
                Profile
              </a>
              <a
                href="/settings"
                className="flex items-center gap-2 px-4 py-2 text-sm text-foreground transition-colors hover:bg-accent"
              >
                <Settings size={14} />
                Settings
              </a>
              <div className="my-1 border-t border-border" />
              <button className="flex w-full items-center gap-2 px-4 py-2 text-sm text-nexus-red-400 transition-colors hover:bg-accent">
                <LogOut size={14} />
                Sign out
              </button>
            </div>
          )}
        </div>
      </div>
    </header>
  );
}
