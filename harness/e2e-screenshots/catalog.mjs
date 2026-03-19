const SUPPORTED_CAPTURE_MODES = new Set(["journey", "full-pages"]);
export const DEFAULT_SCREENSHOT_COMMENT_MARKER = "<!-- 1-tok-e2e-screenshots -->";

const BUSINESS_LABELS = new Map([
  ["home", "Home"],
  ["public", "Public"],
  ["buyer", "Buyer"],
  ["provider", "Provider"],
  ["ops", "Operations"],
]);

const BUSINESS_ORDER = ["home", "public", "buyer", "provider", "ops"];
const DEVICE_ORDER = ["desktop", "mobile"];

export function resolveCaptureMode(rawMode) {
  const mode = String(rawMode ?? "").trim().toLowerCase();
  if (!mode) {
    return "journey";
  }
  if (!SUPPORTED_CAPTURE_MODES.has(mode)) {
    throw new Error(`unknown screenshot capture mode: ${mode}`);
  }
  return mode;
}

export function buildCaptureRunGroups({ mode, deviceKeys }) {
  if (mode === "full-pages") {
    return [{ seedKey: "shared", deviceKeys: [...deviceKeys] }];
  }

  return deviceKeys.map((deviceKey) => ({
    seedKey: deviceKey,
    deviceKeys: [deviceKey],
  }));
}

export function buildFullPagesPlan({
  device,
  rfqId,
  orderId,
  applicationId = "app_1",
}) {
  return [
    { businessLine: "public", device, order: 1, slug: "home", title: "Home", url: "/" },
    { businessLine: "public", device, order: 2, slug: "buyer-login", title: "Buyer login", url: "/login?next=/buyer" },
    { businessLine: "public", device, order: 3, slug: "provider-login", title: "Provider login", url: "/login?next=/provider" },
    { businessLine: "public", device, order: 4, slug: "internal-login", title: "Internal login", url: "/internal/login?next=/ops" },
    { businessLine: "buyer", device, order: 5, slug: "dashboard", title: "Buyer dashboard", url: "/buyer" },
    { businessLine: "buyer", device, order: 6, slug: "post-request", title: "Buyer post request", url: "/buyer/rfqs/create" },
    { businessLine: "buyer", device, order: 7, slug: "order-detail", title: "Buyer order detail", url: `/buyer/orders/${orderId}` },
    { businessLine: "provider", device, order: 8, slug: "dashboard", title: "Provider dashboard", url: "/provider" },
    { businessLine: "provider", device, order: 9, slug: "rfqs", title: "Provider RFQs", url: "/provider/rfqs" },
    { businessLine: "provider", device, order: 10, slug: "rfq-detail", title: "Provider RFQ detail", url: `/provider/rfqs/${rfqId}` },
    { businessLine: "provider", device, order: 11, slug: "proposals", title: "Provider proposals", url: "/provider/proposals" },
    { businessLine: "provider", device, order: 12, slug: "order-detail", title: "Provider order detail", url: `/provider/orders/${orderId}` },
    { businessLine: "ops", device, order: 13, slug: "dashboard", title: "Ops dashboard", url: "/ops" },
    { businessLine: "ops", device, order: 14, slug: "applications", title: "Ops applications", url: "/ops/applications" },
    { businessLine: "ops", device, order: 15, slug: "application-detail", title: "Ops application detail", url: `/ops/applications/${applicationId}` },
    { businessLine: "ops", device, order: 16, slug: "disputes", title: "Ops disputes", url: "/ops/disputes" },
  ];
}

export function renderLocalComment(entries) {
  const lines = [
    "# E2E Marketplace Screenshots",
    "",
  ];

  for (const section of buildEntrySections(entries)) {
    lines.push(`## ${section.businessLabel}`, "");

    for (const deviceSection of section.devices) {
      lines.push(`### ${capitalize(deviceSection.deviceKey)}`, "");
      for (const row of deviceSection.entries) {
        lines.push(`- ${row.title}: \`${row.path}\``);
      }
      lines.push("");
    }
  }

  return `${lines.join("\n").trim()}\n`;
}

export function renderGitHubPRComment(
  entries,
  {
    repository,
    refName,
    runUrl,
    artifactUrl = runUrl,
    marker = DEFAULT_SCREENSHOT_COMMENT_MARKER,
  },
) {
  const base = `https://raw.githubusercontent.com/${repository}/${refName}`;
  const lines = [
    marker,
    "## E2E Marketplace Screenshots",
    "",
    `- Run: ${runUrl}`,
    `- Artifact: [e2e-marketplace-screenshots](${artifactUrl})`,
    "",
  ];

  for (const section of buildEntrySections(entries)) {
    lines.push(`### ${section.businessLabel}`, "");

    for (const deviceSection of section.devices) {
      lines.push(`#### ${capitalize(deviceSection.deviceKey)}`, "");

      for (const group of chunk(deviceSection.entries, 3)) {
        lines.push(`| ${group.map((entry) => entry.title).join(" | ")} |`);
        lines.push(`|${group.map(() => ":---:").join("|")}|`);
        lines.push(`| ${group.map((entry) => `![${entry.slug}](${base}/${entry.path})`).join(" | ")} |`);
        lines.push("");
      }
    }
  }

  return `${lines.join("\n").trim()}\n`;
}

function buildEntrySections(entries) {
  const sections = [];

  for (const businessKey of BUSINESS_ORDER) {
    const businessLabel = BUSINESS_LABELS.get(businessKey);
    if (!businessLabel) continue;

    const businessEntries = entries.filter((entry) => entry.businessLine === businessKey);
    if (businessEntries.length === 0) continue;

    const devices = [];
    for (const deviceKey of DEVICE_ORDER) {
      const deviceEntries = businessEntries
        .filter((entry) => entry.device === deviceKey)
        .sort((left, right) => left.order - right.order);

      if (deviceEntries.length === 0) continue;
      devices.push({ deviceKey, entries: deviceEntries });
    }

    if (devices.length === 0) continue;
    sections.push({ businessKey, businessLabel, devices });
  }

  return sections;
}

function chunk(items, size) {
  const groups = [];
  for (let index = 0; index < items.length; index += size) {
    groups.push(items.slice(index, index + size));
  }
  return groups;
}

function capitalize(value) {
  return value.charAt(0).toUpperCase() + value.slice(1);
}
