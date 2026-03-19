import * as React from "react";

import { cn } from "@/lib/utils";

const Input = React.forwardRef<HTMLInputElement, React.ComponentProps<"input">>(({ className, type, ...props }, ref) => {
  return (
    <input
      type={type}
      ref={ref}
      className={cn(
        "flex h-12 w-full rounded-none border-0 border-b border-input bg-transparent px-0 py-2 text-sm text-foreground outline-none transition-[border-color,color] duration-150 focus-visible:border-foreground focus-visible:ring-0 disabled:cursor-not-allowed disabled:opacity-50",
        className,
      )}
      {...props}
    />
  );
});
Input.displayName = "Input";

export { Input };
