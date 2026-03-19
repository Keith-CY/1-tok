"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  RiAddLine,
  RiFileList3Line,
  RiFolderChartLine,
  RiShieldCheckLine,
  RiStore2Line,
  RiTaskLine,
} from "react-icons/ri";

import { cn } from "@/lib/utils";

const iconMap = {
  add: RiAddLine,
  fileList: RiFileList3Line,
  folderChart: RiFolderChartLine,
  shieldCheck: RiShieldCheckLine,
  store: RiStore2Line,
  task: RiTaskLine,
} as const;

export type WorkspaceNavItem = {
  href: string;
  label: string;
  icon: keyof typeof iconMap;
};

export function WorkspaceNav({
  items,
  mobile = false,
}: {
  items: WorkspaceNavItem[];
  mobile?: boolean;
}) {
  const pathname = usePathname();
  const activeHref =
    items
      .filter((item) => pathname === item.href || pathname?.startsWith(`${item.href}/`))
      .sort((left, right) => right.href.length - left.href.length)[0]?.href ?? null;

  return (
    <nav
      className={cn(
        mobile ? "flex gap-2 overflow-x-auto pb-1" : "grid gap-1.5",
      )}
      aria-label="Workspace"
    >
      {items.map((item) => {
        const Icon = iconMap[item.icon];
        const active = item.href === activeHref;

        return (
          <Link
            key={`${item.href}:${item.label}`}
            href={item.href}
            className={cn(
              "group inline-flex items-center gap-3 rounded-full px-3 py-2.5 text-sm font-medium transition-colors duration-150",
              mobile ? "shrink-0" : "w-full",
              active
                ? "bg-[var(--ink-accent-weak)] text-foreground"
                : "text-muted-foreground hover:bg-secondary/85 hover:text-foreground",
            )}
            aria-current={active ? "page" : undefined}
          >
            <Icon className={cn("size-4 transition-colors duration-150", active ? "text-primary" : "text-muted-foreground group-hover:text-primary")} />
            <span>{item.label}</span>
          </Link>
        );
      })}
    </nav>
  );
}
