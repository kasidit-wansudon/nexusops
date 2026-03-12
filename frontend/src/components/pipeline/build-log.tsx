"use client";

import React, { useEffect, useRef, useState, useCallback } from "react";
import {
  ArrowDown,
  Copy,
  Check,
  Search,
  X,
  Maximize2,
  Minimize2,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";

export interface LogLine {
  lineNumber: number;
  timestamp?: string;
  content: string;
  level?: "info" | "warn" | "error" | "debug";
}

interface BuildLogProps {
  lines: LogLine[];
  title?: string;
  loading?: boolean;
  streaming?: boolean;
  className?: string;
  onLoadMore?: () => void;
  maxHeight?: string;
}

function parseAnsiColors(text: string): React.ReactNode {
  const segments: React.ReactNode[] = [];
  // Simple ANSI parsing for common color codes
  const ansiRegex = /\x1b\[(\d+(?:;\d+)*)m/g;
  let lastIndex = 0;
  let currentClass = "";
  let match;

  const colorMap: Record<string, string> = {
    "30": "ansi-black",
    "31": "ansi-red",
    "32": "ansi-green",
    "33": "ansi-yellow",
    "34": "ansi-blue",
    "35": "ansi-magenta",
    "36": "ansi-cyan",
    "37": "ansi-white",
    "1": "ansi-bold",
    "2": "ansi-dim",
    "0": "",
  };

  const plainText = text.replace(ansiRegex, "");

  if (plainText === text) {
    return text;
  }

  while ((match = ansiRegex.exec(text)) !== null) {
    if (match.index > lastIndex) {
      const content = text.slice(lastIndex, match.index).replace(/\x1b\[[^m]*m/g, "");
      if (content) {
        segments.push(
          <span key={lastIndex} className={currentClass}>
            {content}
          </span>
        );
      }
    }
    const codes = match[1].split(";");
    const classes = codes.map((c) => colorMap[c] || "").filter(Boolean);
    currentClass = classes.join(" ");
    if (match[1] === "0") currentClass = "";
    lastIndex = match.index + match[0].length;
  }

  if (lastIndex < text.length) {
    const remaining = text.slice(lastIndex).replace(/\x1b\[[^m]*m/g, "");
    if (remaining) {
      segments.push(
        <span key={lastIndex} className={currentClass}>
          {remaining}
        </span>
      );
    }
  }

  return <>{segments}</>;
}

function levelColor(level?: string): string {
  switch (level) {
    case "error":
      return "text-nexus-red-400";
    case "warn":
      return "text-yellow-400";
    case "debug":
      return "text-nexus-gray-400";
    default:
      return "text-nexus-gray-200";
  }
}

export function BuildLog({
  lines,
  title = "Build Output",
  loading,
  streaming,
  className,
  maxHeight = "500px",
}: BuildLogProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [autoScroll, setAutoScroll] = useState(true);
  const [copied, setCopied] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [showSearch, setShowSearch] = useState(false);
  const [expanded, setExpanded] = useState(false);

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
  }, [lines, autoScroll]);

  const scrollToBottom = () => {
    if (containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight;
      setAutoScroll(true);
    }
  };

  const copyLogs = async () => {
    const text = lines.map((l) => l.content).join("\n");
    await navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const filteredLines = searchQuery
    ? lines.filter((l) =>
        l.content.toLowerCase().includes(searchQuery.toLowerCase())
      )
    : lines;

  return (
    <div
      className={cn(
        "rounded-lg border border-border bg-nexus-gray-950 overflow-hidden",
        className
      )}
    >
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-2 border-b border-border bg-nexus-gray-900/50">
        <div className="flex items-center gap-2">
          <div className="flex gap-1.5">
            <div className="h-3 w-3 rounded-full bg-nexus-red-500/80" />
            <div className="h-3 w-3 rounded-full bg-yellow-500/80" />
            <div className="h-3 w-3 rounded-full bg-nexus-green-500/80" />
          </div>
          <span className="text-xs font-medium text-muted-foreground ml-2">
            {title}
          </span>
          {streaming && (
            <span className="flex items-center gap-1 text-xs text-nexus-green-400">
              <span className="h-1.5 w-1.5 rounded-full bg-nexus-green-400 animate-pulse-dot" />
              Live
            </span>
          )}
        </div>
        <div className="flex items-center gap-1">
          {showSearch && (
            <div className="flex items-center gap-1 mr-2">
              <input
                type="text"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder="Search logs..."
                className="h-6 w-40 rounded bg-nexus-gray-800 border border-border px-2 text-xs text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
                autoFocus
              />
              <Button
                variant="ghost"
                size="icon-sm"
                onClick={() => {
                  setShowSearch(false);
                  setSearchQuery("");
                }}
                className="h-6 w-6"
              >
                <X className="h-3 w-3" />
              </Button>
            </div>
          )}
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={() => setShowSearch(!showSearch)}
            className="h-6 w-6"
          >
            <Search className="h-3 w-3" />
          </Button>
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={copyLogs}
            className="h-6 w-6"
          >
            {copied ? (
              <Check className="h-3 w-3 text-nexus-green-400" />
            ) : (
              <Copy className="h-3 w-3" />
            )}
          </Button>
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={() => setExpanded(!expanded)}
            className="h-6 w-6"
          >
            {expanded ? (
              <Minimize2 className="h-3 w-3" />
            ) : (
              <Maximize2 className="h-3 w-3" />
            )}
          </Button>
        </div>
      </div>

      {/* Log Content */}
      <div
        ref={containerRef}
        onScroll={handleScroll}
        className="overflow-auto scrollbar-thin terminal-output"
        style={{ maxHeight: expanded ? "80vh" : maxHeight }}
      >
        <div className="p-4 min-w-0">
          {filteredLines.length === 0 && !loading && (
            <div className="text-muted-foreground text-xs text-center py-8">
              {searchQuery ? "No matching log lines" : "No log output yet"}
            </div>
          )}
          {filteredLines.map((line) => (
            <div
              key={line.lineNumber}
              className={cn(
                "flex gap-3 hover:bg-white/[0.02] px-1 -mx-1 rounded",
                levelColor(line.level)
              )}
            >
              <span className="select-none text-muted-foreground/40 min-w-[3ch] text-right shrink-0">
                {line.lineNumber}
              </span>
              {line.timestamp && (
                <span className="select-none text-muted-foreground/60 shrink-0">
                  {line.timestamp}
                </span>
              )}
              <span className="whitespace-pre-wrap break-all">
                {parseAnsiColors(line.content)}
              </span>
            </div>
          ))}
          {loading && (
            <div className="flex items-center gap-2 text-muted-foreground text-xs mt-2">
              <span className="h-1.5 w-1.5 rounded-full bg-nexus-blue-400 animate-pulse-dot" />
              Loading...
            </div>
          )}
        </div>
      </div>

      {/* Scroll to bottom button */}
      {!autoScroll && (
        <div className="absolute bottom-4 right-4">
          <Button
            variant="secondary"
            size="icon-sm"
            onClick={scrollToBottom}
            className="rounded-full shadow-lg"
          >
            <ArrowDown className="h-3 w-3" />
          </Button>
        </div>
      )}
    </div>
  );
}

export default BuildLog;
