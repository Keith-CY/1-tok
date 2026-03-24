import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";

import { cn } from "@/lib/utils";

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap px-4 py-2 text-sm font-semibold tracking-[0.02em] transition-[background-color,color,outline-color,text-decoration-color] duration-150 disabled:pointer-events-none disabled:opacity-50 outline-none focus-visible:outline focus-visible:outline-1 focus-visible:outline-ring",
  {
    variants: {
      variant: {
        default: "bg-primary text-primary-foreground hover:bg-[#111111]",
        secondary: "bg-secondary text-secondary-foreground hover:bg-[var(--surface-highest)]",
        outline: "bg-transparent text-foreground outline outline-1 outline-black/20 outline-offset-[-1px] hover:bg-[var(--surface-lowest)]",
        ghost: "bg-transparent px-0 text-muted-foreground underline-offset-4 hover:text-foreground hover:underline",
        soft: "bg-[var(--accent-weak)] text-accent hover:bg-[var(--accent-medium)]",
        destructive: "bg-destructive text-destructive-foreground hover:bg-[#731818]",
      },
      size: {
        default: "min-h-11",
        sm: "min-h-9 px-3 text-xs uppercase tracking-[0.18em]",
        lg: "min-h-12 px-6",
        icon: "size-10 px-0",
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
