"use client";

import { useState } from "react";
import { Github, Zap } from "lucide-react";
import { Button } from "@/components/ui/button";

function GitLabIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      viewBox="0 0 24 24"
      fill="currentColor"
      xmlns="http://www.w3.org/2000/svg"
    >
      <path d="M22.65 14.39L12 22.13 1.35 14.39a.84.84 0 01-.3-.94l1.22-3.78 2.44-7.51A.42.42 0 014.82 2a.43.43 0 01.58 0 .42.42 0 01.11.18l2.44 7.49h8.1l2.44-7.51A.42.42 0 0118.6 2a.43.43 0 01.58 0 .42.42 0 01.11.18l2.44 7.51L23 13.45a.84.84 0 01-.35.94z" />
    </svg>
  );
}

export default function LoginPage() {
  const [loading, setLoading] = useState<"github" | "gitlab" | null>(null);

  const handleOAuth = async (provider: "github" | "gitlab") => {
    setLoading(provider);
    try {
      const response = await fetch(
        `${process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1"}/auth/${provider}/login`,
        {
          method: "GET",
          headers: { "Content-Type": "application/json" },
        }
      );
      const data = await response.json();
      if (data.redirectUrl) {
        window.location.href = data.redirectUrl;
      }
    } catch {
      setLoading(null);
    }
  };

  return (
    <div className="flex min-h-screen">
      {/* Left side - Branding */}
      <div className="relative hidden w-1/2 flex-col justify-between overflow-hidden bg-gradient-to-br from-nexus-gray-950 via-nexus-gray-900 to-nexus-gray-950 p-12 lg:flex">
        {/* Background Pattern */}
        <div className="absolute inset-0 opacity-[0.03]">
          <svg className="h-full w-full" xmlns="http://www.w3.org/2000/svg">
            <defs>
              <pattern
                id="grid"
                width="40"
                height="40"
                patternUnits="userSpaceOnUse"
              >
                <path
                  d="M 40 0 L 0 0 0 40"
                  fill="none"
                  stroke="white"
                  strokeWidth="1"
                />
              </pattern>
            </defs>
            <rect width="100%" height="100%" fill="url(#grid)" />
          </svg>
        </div>

        {/* Decorative gradient orbs */}
        <div className="absolute -left-32 -top-32 h-96 w-96 rounded-full bg-nexus-green-500/10 blur-3xl" />
        <div className="absolute -bottom-32 -right-32 h-96 w-96 rounded-full bg-nexus-blue-500/10 blur-3xl" />

        <div className="relative">
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-primary shadow-lg shadow-primary/20">
              <Zap className="h-5 w-5 text-primary-foreground" />
            </div>
            <span className="text-2xl font-bold tracking-tight">NexusOps</span>
          </div>
        </div>

        <div className="relative space-y-6">
          <h1 className="text-4xl font-bold leading-tight tracking-tight">
            Ship faster.
            <br />
            <span className="text-primary">Monitor everything.</span>
          </h1>
          <p className="max-w-md text-lg text-muted-foreground">
            The modern developer platform for managing your entire deployment
            pipeline from code to production.
          </p>

          <div className="flex flex-col gap-4 pt-4">
            <FeatureItem text="Unified CI/CD pipeline management" />
            <FeatureItem text="Real-time deployment monitoring" />
            <FeatureItem text="Team collaboration & access control" />
            <FeatureItem text="Automated rollbacks & scaling" />
          </div>
        </div>

        <div className="relative text-sm text-muted-foreground">
          <p>Trusted by engineering teams at companies worldwide.</p>
        </div>
      </div>

      {/* Right side - Login form */}
      <div className="flex w-full flex-col items-center justify-center px-8 lg:w-1/2">
        <div className="w-full max-w-sm space-y-8">
          {/* Mobile logo */}
          <div className="flex items-center gap-3 lg:hidden">
            <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-primary">
              <Zap className="h-5 w-5 text-primary-foreground" />
            </div>
            <span className="text-2xl font-bold">NexusOps</span>
          </div>

          <div>
            <h2 className="text-2xl font-bold tracking-tight">Welcome back</h2>
            <p className="mt-2 text-sm text-muted-foreground">
              Sign in to your account to continue
            </p>
          </div>

          <div className="space-y-3">
            <Button
              variant="outline"
              size="xl"
              className="w-full justify-center gap-3 border-border bg-card hover:bg-accent"
              onClick={() => handleOAuth("github")}
              loading={loading === "github"}
              disabled={loading !== null}
            >
              <Github className="h-5 w-5" />
              Continue with GitHub
            </Button>

            <Button
              variant="outline"
              size="xl"
              className="w-full justify-center gap-3 border-border bg-card hover:bg-accent"
              onClick={() => handleOAuth("gitlab")}
              loading={loading === "gitlab"}
              disabled={loading !== null}
            >
              <GitLabIcon className="h-5 w-5 text-orange-400" />
              Continue with GitLab
            </Button>
          </div>

          <div className="relative">
            <div className="absolute inset-0 flex items-center">
              <div className="w-full border-t border-border" />
            </div>
            <div className="relative flex justify-center text-xs uppercase">
              <span className="bg-background px-2 text-muted-foreground">
                Secure authentication
              </span>
            </div>
          </div>

          <p className="text-center text-xs text-muted-foreground">
            By signing in, you agree to our{" "}
            <a href="#" className="text-primary hover:underline">
              Terms of Service
            </a>{" "}
            and{" "}
            <a href="#" className="text-primary hover:underline">
              Privacy Policy
            </a>
            .
          </p>
        </div>
      </div>
    </div>
  );
}

function FeatureItem({ text }: { text: string }) {
  return (
    <div className="flex items-center gap-3">
      <div className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary/20">
        <div className="h-2 w-2 rounded-full bg-primary" />
      </div>
      <span className="text-sm text-foreground/80">{text}</span>
    </div>
  );
}
