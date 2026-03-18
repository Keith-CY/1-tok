import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";

import { cn } from "@/lib/utils";

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium transition-[transform,box-shadow,background-color,border-color,color] duration-150 disabled:pointer-events-none disabled:opacity-50 outline-none focus-visible:ring-2 focus-visible:ring-ring/40 focus-visible:ring-offset-2 focus-visible:ring-offset-background",
  {
    variants: {
      variant: {
        default: "bg-primary text-primary-foreground shadow-[0_12px_28px_-18px_rgba(37,99,235,0.6)] hover:-translate-y-px hover:bg-primary/96 hover:shadow-[0_18px_32px_-18px_rgba(37,99,235,0.72)]",
        secondary: "bg-secondary text-secondary-foreground hover:-translate-y-px hover:bg-secondary/85",
        outline: "border border-border bg-card text-card-foreground shadow-[0_8px_18px_-16px_rgba(15,23,42,0.2)] hover:-translate-y-px hover:border-primary/20 hover:bg-secondary/70 hover:shadow-[0_14px_26px_-18px_rgba(15,23,42,0.24)]",
        ghost: "text-muted-foreground hover:bg-secondary hover:text-foreground",
        soft: "bg-primary/8 text-primary hover:bg-primary/12",
        destructive: "bg-destructive text-destructive-foreground shadow-sm hover:-translate-y-px hover:bg-destructive/92",
      },
      size: {
        default: "h-11 px-4 py-2",
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
