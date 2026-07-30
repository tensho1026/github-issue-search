import * as LabelPrimitive from "@radix-ui/react-label";
import {
  forwardRef,
  type ComponentPropsWithoutRef,
  type ElementRef,
} from "react";

import { cn } from "../../shared/lib/cn";

export const Label = forwardRef<
  ElementRef<typeof LabelPrimitive.Root>,
  ComponentPropsWithoutRef<typeof LabelPrimitive.Root>
>(({ className, ...props }, ref) => (
  <LabelPrimitive.Root
    className={cn(
      "text-sm leading-none font-semibold text-foreground peer-disabled:cursor-not-allowed peer-disabled:opacity-60",
      className,
    )}
    ref={ref}
    {...props}
  />
));

Label.displayName = LabelPrimitive.Root.displayName;
