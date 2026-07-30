import type { HTMLAttributes, ReactNode } from "react";

import { cn } from "../../shared/lib/cn";
import { Label } from "./label";

type FieldProps = HTMLAttributes<HTMLDivElement> & {
  description?: ReactNode;
  error?: string;
  htmlFor: string;
  label: ReactNode;
};

export function Field({
  children,
  className,
  description,
  error,
  htmlFor,
  label,
  ...props
}: FieldProps) {
  const descriptionId = description ? `${htmlFor}-description` : undefined;
  const errorId = error ? `${htmlFor}-error` : undefined;
  return (
    <div className={cn("grid gap-2.5", className)} {...props}>
      <Label htmlFor={htmlFor}>{label}</Label>
      {children}
      {description ? (
        <p
          className="text-sm leading-6 text-muted-foreground"
          id={descriptionId}
        >
          {description}
        </p>
      ) : null}
      {error ? (
        <p
          className="flex items-start gap-2 text-sm leading-6 font-medium text-danger"
          id={errorId}
          role="alert"
        >
          {error}
        </p>
      ) : null}
    </div>
  );
}
