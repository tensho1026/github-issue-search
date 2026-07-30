import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import { forwardRef, type ButtonHTMLAttributes } from "react";

import { cn } from "../../shared/lib/cn";

const buttonVariants = cva(
  "inline-flex min-h-11 items-center justify-center gap-2 rounded-full text-sm font-semibold tracking-[-0.01em] transition-[color,background-color,border-color,box-shadow,transform] duration-200 outline-none select-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:pointer-events-none disabled:opacity-45 motion-safe:active:translate-y-px",
  {
    defaultVariants: {
      size: "default",
      variant: "primary",
    },
    variants: {
      size: {
        default: "px-5 py-2.5",
        icon: "size-11 p-0",
        large: "min-h-13 px-7 py-3 text-base",
        small: "min-h-9 px-4 py-2 text-xs",
      },
      variant: {
        danger: "bg-danger text-danger-foreground shadow-sm hover:bg-danger/90",
        ghost: "text-muted-foreground hover:bg-muted hover:text-foreground",
        outline:
          "border border-border bg-surface/80 text-foreground shadow-sm hover:border-accent/45 hover:bg-accent-soft",
        primary:
          "bg-accent text-accent-foreground shadow-[0_12px_30px_-14px_var(--accent-glow)] hover:-translate-y-0.5 hover:bg-accent-strong",
        secondary:
          "border border-border bg-muted text-foreground hover:border-accent/35 hover:bg-muted-strong",
      },
    },
  },
);

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean;
  };

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  (
    { asChild = false, className, size, type = "button", variant, ...props },
    ref,
  ) => {
    const Component = asChild ? Slot : "button";
    return (
      <Component
        className={cn(buttonVariants({ className, size, variant }))}
        ref={ref}
        type={asChild ? undefined : type}
        {...props}
      />
    );
  },
);

Button.displayName = "Button";
