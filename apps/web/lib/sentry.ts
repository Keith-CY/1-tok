type SentryInitOptions = {
  dsn?: string;
  enabled: boolean;
  environment: string;
  release?: string;
  tracesSampleRate: number;
  sendDefaultPii: boolean;
};

export function buildBrowserSentryOptions(env: Record<string, string | undefined> = process.env): SentryInitOptions {
  const dsn = env.NEXT_PUBLIC_SENTRY_DSN?.trim();
  return {
    dsn,
    enabled: Boolean(dsn),
    environment: normalizeEnvironment(env.SENTRY_ENVIRONMENT),
    release: env.SENTRY_RELEASE?.trim(),
    tracesSampleRate: parseSampleRate(env.SENTRY_TRACES_SAMPLE_RATE, 0),
    sendDefaultPii: false,
  };
}

export function buildServerSentryOptions(env: Record<string, string | undefined> = process.env): SentryInitOptions {
  const dsn = env.SENTRY_DSN?.trim();
  return {
    dsn,
    enabled: Boolean(dsn),
    environment: normalizeEnvironment(env.SENTRY_ENVIRONMENT),
    release: env.SENTRY_RELEASE?.trim(),
    tracesSampleRate: parseSampleRate(env.SENTRY_TRACES_SAMPLE_RATE, 0),
    sendDefaultPii: false,
  };
}

function normalizeEnvironment(value: string | undefined): string {
  const normalized = value?.trim();
  if (normalized) {
    return normalized;
  }
  return process.env.NODE_ENV === "production" ? "production" : "development";
}

function parseSampleRate(value: string | undefined, fallback: number): number {
  if (!value) {
    return fallback;
  }
  const parsed = Number(value);
  if (Number.isNaN(parsed) || parsed < 0 || parsed > 1) {
    return fallback;
  }
  return parsed;
}
