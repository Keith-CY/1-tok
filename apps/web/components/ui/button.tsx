import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";

import { cn } from "@/lib/utils";

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-full text-sm font-semibold transition-[background-color,border-color,color,box-shadow] duration-150 disabled:pointer-events-none disabled:opacity-50 outline-none focus-visible:ring-2 focus-visible:ring-ring/20 focus-visible:ring-offset-2 focus-visible:ring-offset-background",
  {
    variants: {
      variant: {
        default: "bg-foreground text-background shadow-[0_20px_40px_-28px_rgba(26,28,28,0.46)] hover:bg-foreground/92",
        secondary: "bg-secondary text-secondary-foreground hover:bg-secondary/86",
        outline: "border border-border bg-background/70 text-foreground hover:bg-[var(--ink-accent-weak)] hover:text-foreground",
        ghost: "text-muted-foreground hover:bg-[var(--ink-accent-weak)] hover:text-foreground",
        soft: "bg-[var(--ink-accent-weak)] text-foreground hover:bg-[var(--ink-accent-medium)]",
        destructive: "bg-destructive text-destructive-foreground shadow-sm hover:bg-destructive/92",
      },
      size: {
        default: "h-11 px-4 py-2.5",
        sm: "h-9 px-3 text-xs",
        lg: "h-12 px-5",
        icon: "size-10",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  },
);

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  asChild?: boolean;
}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, asChild = false, ...props }, ref) => {
    const Comp = asChild ? Slot : "button";
    return <Comp className={cn(buttonVariants({ variant, size, className }))} ref={ref} {...props} />;
  },
);
Button.displayName = "Button";

export { Button, buttonVariants };
