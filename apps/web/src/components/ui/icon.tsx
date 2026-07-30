import type { LucideIcon } from "lucide-react";
import type { ComponentProps } from "react";

import { cn } from "../../shared/lib/cn";

type IconProps = Omit<ComponentProps<LucideIcon>, "aria-label"> & {
  icon: LucideIcon;
  label?: string;
};

export function Icon({
  className,
  icon: IconComponent,
  label,
  ...props
}: IconProps) {
  return (
    <IconComponent
      aria-hidden={label ? undefined : true}
      aria-label={label}
      className={cn("size-4 shrink-0", className)}
      role={label ? "img" : undefined}
      {...props}
    />
  );
}
