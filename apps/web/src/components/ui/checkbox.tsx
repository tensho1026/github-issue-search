import { forwardRef, type InputHTMLAttributes } from "react";

import { cn } from "../../shared/lib/cn";

type CheckboxProps = Omit<InputHTMLAttributes<HTMLInputElement>, "type">;

export const Checkbox = forwardRef<HTMLInputElement, CheckboxProps>(
  ({ className, ...props }, ref) => (
    <input
      className={cn(
        "size-5 shrink-0 cursor-pointer rounded border border-input bg-surface accent-accent outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:cursor-not-allowed disabled:opacity-50",
        className,
      )}
      ref={ref}
      type="checkbox"
      {...props}
    />
  ),
);

Checkbox.displayName = "Checkbox";
