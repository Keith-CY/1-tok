import Link from "next/link";
import type { ReactNode } from "react";

import { SiteHeader } from "./site-header";

interface PortalShellProps {
  eyebrow: string;
  title: string;
  copy: string;
  signal: string;
  asideTitle: string;
  asideItems: Array<{ label: string; value: string; tone?: "default" | "mint" | "warning" | "danger" }>;
  quickActions?: Array<{ label: string; href: string; tone?: "primary" | "secondary" }>;
  children: ReactNode;
}

export function PortalShell({
  eyebrow,
  title,
  copy,
  signal,
  asideTitle,
  asideItems,
  quickActions = [],
  children,
}: PortalShellProps) {
  return (
    <main className="page-frame">
      <SiteHeader />

      <section className="portal-hero">
        <div className="hero__intro portal-hero__copy">
          <span className="eyebrow">{eyebrow}</span>
          <h1 className="portal-title">{title}</h1>
          <p className="section-copy">{copy}</p>
          <span className="signal-pill">{signal}</span>
          {quickActions.length > 0 ? (
            <div className="portal-actions">
              {quickActions.map((action) => (
                <Link
                  key={action.label}
                  href={action.href}
                  className={`portal-action-btn ${
                    action.tone === "primary" ? "portal-action-btn--primary" : "portal-action-btn--secondary"
                  }`}
                >
                  {action.label}
                </Link>
              ))}
            </div>
          ) : null}
        </div>

        <aside className="hero__panel portal-hero__panel">
          <div className="label">{asideTitle}</div>
          <div className="signal-stack">
            {asideItems.map((item) => {
              const className =
                item.tone === "mint"
                  ? "signal-row__value signal-row__value--accent"
                  : item.tone === "warning"
                    ? "signal-row__value signal-row__value--warning"
                    : item.tone === "danger"
                      ? "signal-row__value signal-row__value--warning"
                      : "signal-row__value";

              return (
                <div className="signal-row" key={item.label}>
                  <span className="signal-row__label">{item.label}</span>
                  <span className={className}>{item.value}</span>
                </div>
              );
            })}
          </div>
        </aside>
      </section>

      <section className="section-block">{children}</section>
    </main>
  );
}

