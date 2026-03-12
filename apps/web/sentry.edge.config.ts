import * as Sentry from "@sentry/nextjs";

import { buildServerSentryOptions } from "./lib/sentry";

const options = buildServerSentryOptions();

if (options.enabled) {
  Sentry.init(options);
}
