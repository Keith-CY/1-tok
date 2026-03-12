import { withSentryConfig } from "@sentry/nextjs";

/** @type {import('next').NextConfig} */
const nextConfig = {
  transpilePackages: ["@1tok/contracts"],
  output: "standalone",
};

export default withSentryConfig(nextConfig, {
  disableLogger: true,
  silent: true,
});
