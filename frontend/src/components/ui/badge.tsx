import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const badgeVariants = cva(
  "inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold transition-colors focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2",
  {
    variants: {
      variant: {
        default:
          "border-transparent bg-primary text-primary-foreground shadow",
        secondary:
          "border-transparent bg-secondary text-secondary-foreground",
        success:
          "border-nexus-green-400/20 bg-nexus-green-400/10 text-nexus-green-400",
        warning:
          "border-yellow-400/20 bg-yellow-400/10 text-yellow-400",
        error:
          "border-nexus-red-400/20 bg-nexus-red-400/10 text-nexus-red-400",
        info:
          "border-nexus-blue-400/20 bg-nexus-blue-400/10 text-nexus-blue-400",
        outline: "text-foreground border-border",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
);

export interface BadgeProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof badgeVariants> {
  dot?: boolean;
}

function Badge({ className, variant, dot, children, ...props }: BadgeProps) {
  return (
    <div className={cn(badgeVariants({ variant }), className)} {...props}>
      {dot && (
        <span
          className={cn(
            "mr-1.5 h-1.5 w-1.5 rounded-full",
            variant === "success" && "bg-nexus-green-400",
            variant === "warning" && "bg-yellow-400",
            variant === "error" && "bg-nexus-red-400",
            variant === "info" && "bg-nexus-blue-400",
            (!variant || variant === "default") && "bg-primary",
            variant === "secondary" && "bg-secondary-foreground",
            variant === "outline" && "bg-foreground"
          )}
        />
      )}
      {children}
    </div>
  );
}

export { Badge, badgeVariants };
