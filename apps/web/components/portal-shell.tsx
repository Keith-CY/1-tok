import Link from "next/link";
import type { ReactNode } from "react";

import { Glyph, PortalPoster, resolveGlyphFromText, resolvePortalTheme } from "./portal-visuals";
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
  const theme = resolvePortalTheme(eyebrow);

  return (
    <main className="page-frame">
      <SiteHeader />

      <section className="portal-hero">
        <div className="hero__intro portal-hero__copy">
          <div className="portal-heading">
            <span className="eyebrow">{eyebrow}</span>
            <h1 className="portal-title">{title}</h1>
            <p className="section-copy portal-copy--compact">{copy}</p>
          </div>
          <div className="portal-kicker-row">
            <span className="signal-pill">{signal}</span>
            <span className="portal-kicker-chip">
              <Glyph name={theme === "buyer" ? "buyer" : theme === "provider" ? "provider" : "ops"} className="portal-kicker-chip__icon" />
              visual board
            </span>
          </div>
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
                  <Glyph name={resolveGlyphFromText(action.label)} className="portal-action-btn__icon" />
                  {action.label}
                </Link>
              ))}
            </div>
          ) : null}
        </div>

        <div className="portal-hero__visual-wrap">
          <PortalPoster theme={theme} signal={signal} actions={quickActions} />
        </div>

        <aside className="hero__panel portal-hero__panel">
          <div className="label">{asideTitle}</div>
          <div className="signal-grid">
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
                <div className="signal-card" key={item.label}>
                  <div className="signal-card__head">
                    <span className="signal-card__glyph">
                      <Glyph name={resolveGlyphFromText(item.label)} className="signal-card__glyph-icon" />
                    </span>
                    <span className={className}>{item.value}</span>
                  </div>
                  <span className="signal-row__label">{item.label}</span>
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
