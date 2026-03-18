import * as React from "react";

import { cn } from "@/lib/utils";

function Progress({ className, value = 0 }: { className?: string; value?: number }) {
  return (
    <div className={cn("relative h-2.5 w-full overflow-hidden rounded-full bg-secondary", className)}>
      <div
        className="h-full rounded-full bg-primary transition-all"
        style={{ width: `${Math.max(0, Math.min(100, value))}%` }}
      />
    </div>
  );
}

export { Progress };
