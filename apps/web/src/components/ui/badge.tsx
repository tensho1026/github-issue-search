import { cva, type VariantProps } from "class-variance-authority";
import type { HTMLAttributes } from "react";

import { cn } from "../../shared/lib/cn";

const badgeVariants = cva(
  "inline-flex min-h-7 items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs leading-none font-semibold",
  {
    defaultVariants: {
      variant: "neutral",
    },
    variants: {
      variant: {
        accent: "border-accent/20 bg-accent-soft text-accent-soft-foreground",
        danger: "border-danger/20 bg-danger-soft text-danger",
        info: "border-info/20 bg-info-soft text-info",
        neutral: "border-border bg-muted text-muted-foreground",
        success: "border-success/20 bg-success-soft text-success",
        warning: "border-warning/20 bg-warning-soft text-warning",
      },
    },
  },
);

type BadgeProps = HTMLAttributes<HTMLSpanElement> &
  VariantProps<typeof badgeVariants>;

export function Badge({ className, variant, ...props }: BadgeProps) {
  return (
    <span className={cn(badgeVariants({ className, variant }))} {...props} />
  );
}
