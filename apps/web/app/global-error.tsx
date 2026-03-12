"use client";

import * as Sentry from "@sentry/nextjs";
import { useEffect } from "react";

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    Sentry.captureException(error);
  }, [error]);

  return (
    <html lang="en">
      <body>
        <main className="site-shell">
          <section className="portal-shell">
            <div className="portal-shell__header">
              <p className="portal-shell__eyebrow">Control room unavailable</p>
              <h1>Something failed while rendering this view.</h1>
              <p>Ops has the exception. Retry once before escalating.</p>
            </div>
            <button type="button" onClick={() => reset()}>
              Retry render
            </button>
          </section>
        </main>
      </body>
    </html>
  );
}
