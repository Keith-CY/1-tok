import * as React from "react";

import { cn } from "@/lib/utils";

const Input = React.forwardRef<HTMLInputElement, React.ComponentProps<"input">>(({ className, type, ...props }, ref) => {
  return (
    <input
      type={type}
      ref={ref}
      className={cn(
        "flex min-h-11 w-full border-x-0 border-t-0 border-b border-transparent bg-secondary px-0 py-3 text-sm text-foreground outline-none transition-[background-color,border-color] duration-150 focus-visible:border-b-primary focus-visible:bg-card disabled:cursor-not-allowed disabled:bg-[var(--surface-highest)] disabled:opacity-60",
        className,
      )}
      {...props}
    />
  );
});
Input.displayName = "Input";

export { Input };
