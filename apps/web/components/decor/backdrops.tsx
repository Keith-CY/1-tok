import { cn } from "@/lib/utils";

export function FlickeringGrid({ className }: { className?: string }) {
  return (
    <div className={cn("pointer-events-none absolute inset-0 overflow-hidden", className)} aria-hidden="true">
      <div className="absolute inset-0 opacity-[0.18] [background-image:linear-gradient(to_right,rgba(31,64,102,0.14)_1px,transparent_1px),linear-gradient(to_bottom,rgba(31,64,102,0.14)_1px,transparent_1px)] [background-size:24px_24px] [mask-image:radial-gradient(circle_at_center,black,transparent_78%)]" />
      <div className="absolute inset-x-0 top-0 h-40 bg-[radial-gradient(circle_at_top,rgba(95,165,185,0.28),transparent_72%)] animate-pulse" />
      <div className="absolute left-1/3 top-1/4 size-40 rounded-full bg-[radial-gradient(circle,rgba(254,196,116,0.22),transparent_70%)] blur-3xl" />
    </div>
  );
}

export function BackgroundBeams({ className }: { className?: string }) {
  return (
    <div className={cn("pointer-events-none absolute inset-0 overflow-hidden", className)} aria-hidden="true">
      <div className="absolute -left-24 top-12 h-64 w-64 rounded-full bg-[radial-gradient(circle,rgba(64,146,189,0.18),transparent_66%)] blur-3xl" />
      <div className="absolute right-0 top-0 h-[28rem] w-[28rem] bg-[radial-gradient(circle,rgba(255,195,120,0.18),transparent_65%)] blur-3xl" />
      <div className="absolute inset-x-1/4 top-0 h-px bg-gradient-to-r from-transparent via-primary/40 to-transparent" />
      <div className="absolute inset-y-0 right-16 w-px bg-gradient-to-b from-transparent via-primary/25 to-transparent" />
    </div>
  );
}
