import { forwardRef, type InputHTMLAttributes } from "react";

import { cn } from "../../shared/lib/cn";

type SliderProps = Omit<InputHTMLAttributes<HTMLInputElement>, "type">;

export const Slider = forwardRef<HTMLInputElement, SliderProps>(
  ({ className, ...props }, ref) => (
    <input
      className={cn(
        "h-6 w-full cursor-pointer accent-accent outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:cursor-not-allowed disabled:opacity-50",
        className,
      )}
      ref={ref}
      type="range"
      {...props}
    />
  ),
);

Slider.displayName = "Slider";
