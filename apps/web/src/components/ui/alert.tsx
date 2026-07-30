import { cva, type VariantProps } from "class-variance-authority";
import type { HTMLAttributes } from "react";

import { cn } from "../../shared/lib/cn";

const alertVariants = cva(
  "relative grid gap-1 rounded-xl border p-4 text-sm leading-6",
  {
    defaultVariants: {
      variant: "info",
    },
    variants: {
      variant: {
        danger: "border-danger/25 bg-danger-soft text-danger",
        info: "border-info/20 bg-info-soft text-foreground",
        success: "border-success/20 bg-success-soft text-foreground",
        warning: "border-warning/25 bg-warning-soft text-foreground",
      },
    },
  },
);

type AlertProps = HTMLAttributes<HTMLDivElement> &
  VariantProps<typeof alertVariants>;

export function Alert({ className, variant, ...props }: AlertProps) {
  return (
    <div
      className={cn(alertVariants({ className, variant }))}
      role={variant === "danger" ? "alert" : "status"}
      {...props}
    />
  );
}

export function AlertTitle({
  className,
  ...props
}: HTMLAttributes<HTMLHeadingElement>) {
  return (
    <h3
      className={cn("font-semibold tracking-[-0.01em]", className)}
      {...props}
    />
  );
}

export function AlertDescription({
  className,
  ...props
}: HTMLAttributes<HTMLParagraphElement>) {
  return <p className={cn("opacity-85", className)} {...props} />;
}
