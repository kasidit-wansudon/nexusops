"use client";

import React, { useEffect, useRef, useState, useCallback } from "react";
import {
  ArrowDown,
  Copy,
  Check,
  Search,
  X,
  Filter,
  Download,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";

export interface LogEntry {
  id: string;
  timestamp: string;
  message: string;
  level: "info" | "warn" | "error" | "debug" | "trace";
  source?: string;
  metadata?: Record<string, string>;
}

interface LogViewerProps {
  entries: LogEntry[];
  streaming?: boolean;
  title?: string;
  className?: string;
  maxHeight?: string;
  onEntryClick?: (entry: LogEntry) => void;
  showTimestamps?: boolean;
  showLevels?: boolean;
}

const levelConfig = {
  info: {
    color: "text-nexus-blue-400",
    bgColor: "bg-nexus-blue-400/5",
    label: "INF",
  },
  warn: {
    color: "text-yellow-400",
    bgColor: "bg-yellow-400/5",
    label: "WRN",
  },
  error: {
    color: "text-nexus-red-400",
    bgColor: "bg-nexus-red-400/5",
    label: "ERR",
  },
  debug: {
    color: "text-nexus-gray-400",
    bgColor: "bg-nexus-gray-400/5",
    label: "DBG",
  },
  trace: {
    color: "text-nexus-gray-500",
    bgColor: "bg-nexus-gray-500/5",
    label: "TRC",
  },
};

export function LogViewer({
  entries,
  streaming,
  title = "Logs",
  className,
  maxHeight = "600px",
  onEntryClick,
  showTimestamps = true,
  showLevels = true,
}: LogViewerProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [autoScroll, setAutoScroll] = useState(true);
  const [copied, setCopied] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [showSearch, setShowSearch] = useState(false);
  const [levelFilter, setLevelFilter] = useState<Set<string>>(
    new Set(["info", "warn", "error", "debug", "trace"])
  );
  const [showFilter, setShowFilter] = useState(false);

  const handleScroll = useCallback(() => {
    if (!containerRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = containerRef.current;
    const isAtBottom = scrollHeight - scrollTop - clientHeight < 50;
    setAutoScroll(isAtBottom);
  }, []);

  useEffect(() => {
    if (autoScroll && containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight;
    }
  }, [entries, autoScroll]);

  const scrollToBottom = () => {
    if (containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight;
      setAutoScroll(true);
    }
  };

  const copyLogs = async () => {
    const text = filteredEntries
      .map(
        (e) =>
          `${e.timestamp} [${e.level.toUpperCase()}] ${e.source ? `(${e.source}) ` : ""}${e.message}`
      )
      .join("\n");
    await navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const downloadLogs = () => {
    const text = filteredEntries
      .map(
        (e) =>
          `${e.timestamp} [${e.level.toUpperCase()}] ${e.source ? `(${e.source}) ` : ""}${e.message}`
      )
      .join("\n");
    const blob = new Blob([text], { type: "text/plain" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `logs-${new Date().toISOString()}.txt`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const toggleLevel = (level: string) => {
    const next = new Set(levelFilter);
    if (next.has(level)) {
      next.delete(level);
    } else {
      next.add(level);
    }
    setLevelFilter(next);
  };

  const filteredEntries = entries.filter((e) => {
    if (!levelFilter.has(e.level)) return false;
    if (searchQuery) {
      const q = searchQuery.toLowerCase();
      return (
        e.message.toLowerCase().includes(q) ||
        e.source?.toLowerCase().includes(q) ||
        e.timestamp.includes(q)
      );
    }
    return true;
  });

  return (
    <div
      className={cn(
        "rounded-lg border border-border bg-nexus-gray-950 overflow-hidden flex flex-col",
        className
      )}
    >
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-2 border-b border-border bg-nexus-gray-900/50">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium text-foreground">{title}</span>
          {streaming && (
            <span className="flex items-center gap-1 text-xs text-nexus-green-400">
              <span className="h-1.5 w-1.5 rounded-full bg-nexus-green-400 animate-pulse-dot" />
              Streaming
            </span>
          )}
          <span className="text-xs text-muted-foreground">
            {filteredEntries.length} entries
          </span>
        </div>
        <div className="flex items-center gap-1">
          {showSearch && (
            <div className="flex items-center gap-1 mr-2">
              <input
                type="text"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder="Filter logs..."
                className="h-7 w-48 rounded bg-nexus-gray-800 border border-border px-2 text-xs text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
                autoFocus
              />
              <Button
                variant="ghost"
                size="icon-sm"
                onClick={() => {
                  setShowSearch(false);
                  setSearchQuery("");
                }}
                className="h-7 w-7"
              >
                <X className="h-3.5 w-3.5" />
              </Button>
            </div>
          )}
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={() => setShowSearch(!showSearch)}
          >
            <Search className="h-3.5 w-3.5" />
          </Button>
          <div className="relative">
            <Button
              variant="ghost"
              size="icon-sm"
              onClick={() => setShowFilter(!showFilter)}
            >
              <Filter className="h-3.5 w-3.5" />
            </Button>
            {showFilter && (
              <div className="absolute right-0 top-full mt-1 z-10 rounded-md border border-border bg-popover p-2 shadow-md">
                {Object.entries(levelConfig).map(([level, config]) => (
                  <label
                    key={level}
                    className="flex items-center gap-2 px-2 py-1 cursor-pointer hover:bg-accent rounded text-xs"
                  >
                    <input
                      type="checkbox"
                      checked={levelFilter.has(level)}
                      onChange={() => toggleLevel(level)}
                      className="rounded border-border"
                    />
                    <span className={config.color}>{config.label}</span>
                    <span className="text-muted-foreground capitalize">
                      {level}
                    </span>
                  </label>
                ))}
              </div>
            )}
          </div>
          <Button variant="ghost" size="icon-sm" onClick={copyLogs}>
            {copied ? (
              <Check className="h-3.5 w-3.5 text-nexus-green-400" />
            ) : (
              <Copy className="h-3.5 w-3.5" />
            )}
          </Button>
          <Button variant="ghost" size="icon-sm" onClick={downloadLogs}>
            <Download className="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>

      {/* Log Content */}
      <div
        ref={containerRef}
        onScroll={handleScroll}
        className="overflow-auto scrollbar-thin font-mono text-[13px] leading-6"
        style={{ maxHeight }}
      >
        <div className="p-2 min-w-0">
          {filteredEntries.length === 0 && (
            <div className="text-muted-foreground text-xs text-center py-12">
              {searchQuery || levelFilter.size < 5
                ? "No matching log entries"
                : "No log entries yet"}
            </div>
          )}
          {filteredEntries.map((entry) => {
            const config = levelConfig[entry.level];
            return (
              <div
                key={entry.id}
                onClick={() => onEntryClick?.(entry)}
                className={cn(
                  "flex gap-2 px-2 py-0.5 rounded hover:bg-white/[0.03] cursor-default",
                  config.bgColor,
                  onEntryClick && "cursor-pointer"
                )}
              >
                {showTimestamps && (
                  <span className="text-muted-foreground/50 shrink-0 select-none tabular-nums">
                    {entry.timestamp}
                  </span>
                )}
                {showLevels && (
                  <span
                    className={cn(
                      "shrink-0 select-none font-semibold w-[3ch]",
                      config.color
                    )}
                  >
                    {config.label}
                  </span>
                )}
                {entry.source && (
                  <span className="text-muted-foreground shrink-0 select-none">
                    [{entry.source}]
                  </span>
                )}
                <span className="text-foreground/90 whitespace-pre-wrap break-all">
                  {entry.message}
                </span>
              </div>
            );
          })}
        </div>
      </div>

      {/* Scroll to bottom */}
      {!autoScroll && (
        <div className="flex justify-center p-2 border-t border-border bg-nexus-gray-900/30">
          <Button
            variant="ghost"
            size="sm"
            onClick={scrollToBottom}
            className="text-xs"
          >
            <ArrowDown className="h-3 w-3 mr-1" />
            Scroll to bottom
          </Button>
        </div>
      )}
    </div>
  );
}

export default LogViewer;
