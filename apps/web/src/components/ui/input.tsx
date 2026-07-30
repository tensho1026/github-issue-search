import { forwardRef, type InputHTMLAttributes } from "react";

import { cn } from "../../shared/lib/cn";

export const Input = forwardRef<
  HTMLInputElement,
  InputHTMLAttributes<HTMLInputElement>
>(({ "aria-invalid": ariaInvalid, className, ...props }, ref) => (
  <input
    aria-invalid={ariaInvalid}
    className={cn(
      "min-h-12 w-full rounded-xl border border-input bg-surface/90 px-4 text-base text-foreground shadow-sm outline-none transition-[border-color,box-shadow,background-color] placeholder:text-muted-foreground/70 focus-visible:border-accent focus-visible:ring-3 focus-visible:ring-accent/15 disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:border-danger aria-invalid:ring-3 aria-invalid:ring-danger/10",
      className,
    )}
    ref={ref}
    {...props}
  />
));

Input.displayName = "Input";
