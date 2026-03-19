import fs from "node:fs/promises";

import { renderGitHubPRComment, renderLocalComment } from "./catalog.mjs";

const COMMENT_FORMAT = String(process.env.SCREENSHOT_COMMENT_FORMAT || "local").trim().toLowerCase();

async function main() {
  const manifestPath = requiredEnv("MANIFEST_PATH");
  const outputPath = requiredEnv("COMMENT_OUTPUT_PATH");
  const manifest = JSON.parse(await fs.readFile(manifestPath, "utf8"));

  let comment = "";
  if (COMMENT_FORMAT === "local") {
    comment = renderLocalComment(manifest);
  } else if (COMMENT_FORMAT === "github-pr") {
    const repository = requiredEnv("REPOSITORY");
    const refName = requiredEnv("REF_NAME");
    const runId = process.env.RUN_ID;
    const runUrl = process.env.RUN_URL || buildRunUrl(repository, runId);

    comment = renderGitHubPRComment(manifest, {
      repository,
      refName,
      runUrl,
      artifactUrl: process.env.ARTIFACT_URL || runUrl,
      marker: process.env.COMMENT_MARKER,
    });
  } else {
    throw new Error(`unknown screenshot comment format: ${COMMENT_FORMAT}`);
  }

  await fs.writeFile(outputPath, comment, "utf8");
}

function requiredEnv(name) {
  const value = process.env[name];
  if (!value) {
    throw new Error(`missing required environment variable: ${name}`);
  }
  return value;
}

function buildRunUrl(repository, runId) {
  if (!runId) {
    throw new Error("missing RUN_URL or RUN_ID for github-pr comment rendering");
  }
  return `https://github.com/${repository}/actions/runs/${runId}`;
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
