import * as Sentry from "@sentry/nextjs";

import { buildBrowserSentryOptions } from "./lib/sentry";

const options = buildBrowserSentryOptions();

if (options.enabled) {
  Sentry.init(options);
}

export const onRouterTransitionStart = Sentry.captureRouterTransitionStart;
