import { afterEach, describe, expect, it } from "bun:test";

import { buildBrowserSentryOptions, buildServerSentryOptions } from "./sentry";

const env = process.env as Record<string, string | undefined>;

afterEach(() => {
  delete env.NEXT_PUBLIC_SENTRY_DSN;
  delete env.SENTRY_DSN;
  delete env.SENTRY_ENVIRONMENT;
  delete env.SENTRY_RELEASE;
  delete env.SENTRY_TRACES_SAMPLE_RATE;
});

describe("web sentry config", () => {
  it("builds browser options from environment", () => {
    env.NEXT_PUBLIC_SENTRY_DSN = "https://public@example.com/1";
    env.SENTRY_ENVIRONMENT = "production";
    env.SENTRY_RELEASE = "sha-123";
    env.SENTRY_TRACES_SAMPLE_RATE = "0.25";

    const options = buildBrowserSentryOptions();

    expect(options).toMatchObject({
      dsn: "https://public@example.com/1",
      enabled: true,
      environment: "production",
      release: "sha-123",
      tracesSampleRate: 0.25,
    });
    expect("replaysSessionSampleRate" in options).toBe(false);
  });

  it("disables browser sentry when no dsn is configured", () => {
    const options = buildBrowserSentryOptions();
    expect(options.enabled).toBe(false);
  });

  it("builds server options from the private dsn", () => {
    env.SENTRY_DSN = "https://private@example.com/2";
    env.SENTRY_ENVIRONMENT = "staging";
    env.SENTRY_RELEASE = "sha-456";

    const options = buildServerSentryOptions();

    expect(options).toMatchObject({
      dsn: "https://private@example.com/2",
      enabled: true,
      environment: "staging",
      release: "sha-456",
    });
  });
});
