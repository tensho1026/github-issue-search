import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import {
  forwardRef,
  type ComponentPropsWithoutRef,
  type ElementRef,
} from "react";

import { cn } from "../../shared/lib/cn";

export function Tooltip(
  props: ComponentPropsWithoutRef<typeof TooltipPrimitive.Root>,
) {
  return <TooltipPrimitive.Root {...props} />;
}

export function TooltipTrigger(
  props: ComponentPropsWithoutRef<typeof TooltipPrimitive.Trigger>,
) {
  return <TooltipPrimitive.Trigger {...props} />;
}

export function TooltipProvider(
  props: ComponentPropsWithoutRef<typeof TooltipPrimitive.Provider>,
) {
  return <TooltipPrimitive.Provider {...props} />;
}

export const TooltipContent = forwardRef<
  ElementRef<typeof TooltipPrimitive.Content>,
  ComponentPropsWithoutRef<typeof TooltipPrimitive.Content>
>(({ className, sideOffset = 8, ...props }, ref) => (
  <TooltipPrimitive.Portal>
    <TooltipPrimitive.Content
      className={cn(
        "z-50 max-w-64 rounded-lg border border-border bg-foreground px-3 py-2 text-xs leading-5 text-background shadow-xl data-[state=delayed-open]:animate-in data-[state=closed]:animate-out",
        className,
      )}
      ref={ref}
      sideOffset={sideOffset}
      {...props}
    />
  </TooltipPrimitive.Portal>
));

TooltipContent.displayName = TooltipPrimitive.Content.displayName;
