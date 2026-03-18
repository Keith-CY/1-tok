import fs from "node:fs/promises";
import path from "node:path";
import { chromium, devices } from "playwright";

const OUTPUT_DIR = process.env.SCREENSHOT_OUTPUT_DIR || "/artifacts";
const WEB_BASE_URL = trimTrailingSlash(process.env.SCREENSHOT_WEB_BASE_URL || "http://web:3000");
const API_BASE_URL = trimTrailingSlash(process.env.SCREENSHOT_API_BASE_URL || "http://api-gateway:8080");
const IAM_BASE_URL = trimTrailingSlash(process.env.SCREENSHOT_IAM_BASE_URL || "http://iam:8081");
const SETTLEMENT_BASE_URL = trimTrailingSlash(process.env.SCREENSHOT_SETTLEMENT_BASE_URL || "http://settlement:8083");
const SETTLEMENT_SERVICE_TOKEN = process.env.SCREENSHOT_SETTLEMENT_SERVICE_TOKEN || "local-settlement-service-token";
const EXECUTION_BASE_URL = trimTrailingSlash(process.env.SCREENSHOT_EXECUTION_BASE_URL || "http://execution:8085");
const EXECUTION_EVENT_TOKEN = process.env.SCREENSHOT_EXECUTION_EVENT_TOKEN || "local-execution-event-token";
const PASSWORD = "correct horse battery staple 123";
const SERVICE_TOKEN_HEADER = "X-One-Tok-Service-Token";

const DEVICE_PRESETS = [
  {
    key: "desktop",
    contextOptions: {
      viewport: { width: 1440, height: 1024 },
      colorScheme: "light",
    },
  },
  {
    key: "mobile",
    contextOptions: {
      ...devices["iPhone 12"],
      colorScheme: "light",
    },
  },
];

const manifest = [];

async function main() {
  await fs.mkdir(OUTPUT_DIR, { recursive: true });
  const browser = await chromium.launch({ headless: true });

  try {
    for (const device of DEVICE_PRESETS) {
      await runFlowForDevice(browser, device);
    }

    await fs.writeFile(path.join(OUTPUT_DIR, "manifest.json"), JSON.stringify(manifest, null, 2) + "\n", "utf8");
    await fs.writeFile(path.join(OUTPUT_DIR, "comment.md"), renderLocalComment(manifest), "utf8");
    console.log(`screenshots saved to ${OUTPUT_DIR}`);
  } finally {
    await browser.close();
  }
}

async function runFlowForDevice(browser, device) {
  const suffix = `${device.key}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
  const buyer = await createUser("buyer", suffix);
  const provider = await createUser("provider", suffix);
  const requestTitle = `Live request ${suffix}`;
  const requestBudgetCents = 285000;
  const quoteCents = 241000;

  const homeContext = await browser.newContext(device.contextOptions);
  const homePage = await homeContext.newPage();
  await goto(homePage, `${WEB_BASE_URL}/`);
  await capture(homePage, { businessLine: "home", device: device.key, order: 1, slug: "home", title: "Home" });
  await homeContext.close();

  const buyerContext = await browser.newContext(device.contextOptions);
  const buyerPage = await buyerContext.newPage();
  const providerContext = await browser.newContext(device.contextOptions);
  const providerPage = await providerContext.newPage();

  try {
    await goto(buyerPage, `${WEB_BASE_URL}/login?next=/buyer`);
    await capture(buyerPage, { businessLine: "buyer", device: device.key, order: 1, slug: "login", title: "Buyer login" });
    await login(buyerPage, buyer.email, buyer.password);

    await goto(buyerPage, `${WEB_BASE_URL}/buyer/rfqs/create`);
    await capture(buyerPage, {
      businessLine: "buyer",
      device: device.key,
      order: 2,
      slug: "post-request",
      title: "Buyer post request",
    });
    await createRequest(buyerPage, requestTitle, requestBudgetCents);
    await capture(buyerPage, {
      businessLine: "buyer",
      device: device.key,
      order: 3,
      slug: "manage-requests",
      title: "Buyer manage requests",
    });

    await goto(providerPage, `${WEB_BASE_URL}/login?next=/provider`);
    await capture(providerPage, {
      businessLine: "provider",
      device: device.key,
      order: 1,
      slug: "login",
      title: "Provider login",
    });
    await login(providerPage, provider.email, provider.password);

    await openProviderRequest(providerPage, requestTitle);
    await capture(providerPage, {
      businessLine: "provider",
      device: device.key,
      order: 2,
      slug: "open-request",
      title: "Provider open request",
    });
    await providerPage.locator('input[name="quoteCents"]').fill(String(quoteCents));
    await capture(providerPage, {
      businessLine: "provider",
      device: device.key,
      order: 3,
      slug: "submit-proposal",
      title: "Provider submit proposal",
    });
    await Promise.all([
      providerPage.waitForURL(/\/provider$/),
      providerPage.getByRole("button", { name: /submit proposal/i }).click(),
    ]);
    await waitForRequestBids(buyer.token, requestTitle, 1);

    await goto(buyerPage, `${WEB_BASE_URL}/buyer`);
    await waitForText(buyerPage, requestTitle);
    await capture(buyerPage, {
      businessLine: "buyer",
      device: device.key,
      order: 4,
      slug: "award-request",
      title: "Buyer award request",
    });
    await clickAwardForRequest(buyerPage, requestTitle);
    const orderId = await waitForAwardedOrderId(buyer.token, requestTitle);

    await goto(providerPage, `${WEB_BASE_URL}/provider/proposals`);
    await capture(providerPage, {
      businessLine: "provider",
      device: device.key,
      order: 4,
      slug: "awarded-work",
      title: "Provider awarded work",
    });

    await goto(providerPage, `${WEB_BASE_URL}/provider/orders/${orderId}`);
    await capture(providerPage, {
      businessLine: "provider",
      device: device.key,
      order: 5,
      slug: "delivery-in-progress",
      title: "Provider delivery in progress",
    });

    const settledOrder = await settleOrderFlow(provider.token, orderId);
    await createInvoice(settledOrder);
    await syncSettledFeed();
    await waitForSettledInvoice(provider.token, orderId);

    await goto(providerPage, `${WEB_BASE_URL}/provider/orders/${orderId}`);
    await capture(providerPage, {
      businessLine: "provider",
      device: device.key,
      order: 6,
      slug: "step-complete-and-payout",
      title: "Provider step complete and payout",
    });
    await providerPage.evaluate(() => window.scrollTo(0, document.body.scrollHeight * 0.35));
    await providerPage.waitForTimeout(300);
    await capture(providerPage, {
      businessLine: "provider",
      device: device.key,
      order: 7,
      slug: "delivery-completed",
      title: "Provider delivery completed",
    });

    await goto(buyerPage, `${WEB_BASE_URL}/buyer/orders/${orderId}`);
    await capture(buyerPage, {
      businessLine: "buyer",
      device: device.key,
      order: 5,
      slug: "review-result",
      title: "Buyer review result",
    });
  } finally {
    await buyerContext.close();
    await providerContext.close();
  }
}

async function createUser(kind, suffix) {
  const email = `${kind}-screenshot-${suffix}@example.com`;
  const response = await fetch(`${IAM_BASE_URL}/v1/signup`, {
    method: "POST",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      email,
      password: PASSWORD,
      name: `${capitalize(kind)} Screenshot`,
      organizationName: `${capitalize(kind)} Screenshot Org`,
      organizationKind: kind,
    }),
  });

  if (!response.ok) {
    throw new Error(`create ${kind} user failed: ${response.status}`);
  }

  const payload = await response.json();
  return { email, password: PASSWORD, token: payload.session?.token ?? "" };
}

async function login(page, email, password) {
  await page.locator('input[name="email"]').fill(email);
  await page.locator('input[name="password"]').fill(password);
  await Promise.all([
    page.waitForURL(/\/(buyer|provider)$/),
    page.locator('form[action="/auth/login"] button[type="submit"]').click(),
  ]);
  await waitForHydration(page);
}

async function createRequest(page, title, budgetCents) {
  await page.locator('input[name="title"]').fill(title);
  await page.locator('input[name="budgetCents"]').fill(String(budgetCents));
  await page.locator('input[name="responseDeadlineAt"]').fill(datetimeLocalPlusDays(5));
  await Promise.all([
    page.waitForURL(/\/buyer$/),
    page.getByRole("button", { name: /^post request$/i }).click(),
  ]);
  await waitForHydration(page);
}

async function openProviderRequest(page, title) {
  await goto(page, `${WEB_BASE_URL}/provider/rfqs`);
  await waitForText(page, title);
  const row = page.locator(".market-row", { hasText: title }).first();
  await Promise.all([
    page.waitForURL(/\/provider\/rfqs\//),
    row.getByRole("link", { name: /open request/i }).click(),
  ]);
  await waitForHydration(page);
}

async function clickAwardForRequest(page, title) {
  const row = page.locator(".market-row", { hasText: title }).first();
  await Promise.all([
    page.waitForURL(/\/buyer$/),
    row.getByRole("button", { name: /award low/i }).click(),
  ]);
  await waitForHydration(page);
}

async function waitForRequestBids(token, title, minCount) {
  await poll(async () => {
    const rfqs = await listRFQs(token);
    const item = rfqs.find((candidate) => candidate.title === title);
    if (!item?.id) return false;
    const bids = await listBids(token, item.id);
    return bids.length >= minCount;
  }, 25000, `wait for bids on ${title}`);
}

async function waitForAwardedOrderId(token, title) {
  let awardedOrderId = "";

  await poll(async () => {
    const rfqs = await listRFQs(token);
    const item = rfqs.find((candidate) => candidate.title === title);
    if (!item?.orderId) return false;
    awardedOrderId = item.orderId;
    return true;
  }, 25000, `wait for awarded order on ${title}`);

  return awardedOrderId;
}

async function settleOrderFlow(token, orderId) {
  await settleOrderMilestone(orderId);

  let settledOrder = null;
  await poll(async () => {
    const order = await getOrderById(token, orderId);
    if (!order) return false;
    if (!order.milestones.every((milestone) => milestone.state === "settled")) return false;
    settledOrder = order;
    return true;
  }, 30000, `wait for settled milestones on ${orderId}`);

  return settledOrder;
}

async function waitForSettledInvoice(token, orderId) {
  await poll(async () => {
    const fundingRecords = await listFundingRecords(token);
    return fundingRecords.some((record) => record.orderId === orderId && record.kind === "invoice");
  }, 30000, `wait for settled invoice on ${orderId}`);
}

async function settleOrderMilestone(orderId) {
  const response = await fetch(`${EXECUTION_BASE_URL}/v1/carrier/events`, {
    method: "POST",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
      [SERVICE_TOKEN_HEADER]: EXECUTION_EVENT_TOKEN,
    },
    body: JSON.stringify({
      orderId,
      milestoneId: "ms_1",
      eventType: "milestone_ready",
      summary: "Screenshot harness completed the mocked provider step.",
    }),
  });

  if (!response.ok) {
    throw new Error(`settle order milestone failed: ${response.status}`);
  }
}

async function createInvoice(order) {
  const response = await fetch(`${SETTLEMENT_BASE_URL}/v1/invoices`, {
    method: "POST",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
      [SERVICE_TOKEN_HEADER]: SETTLEMENT_SERVICE_TOKEN,
    },
    body: JSON.stringify({
      orderId: order.id,
      milestoneId: "ms_1",
      buyerOrgId: order.buyerOrgId,
      providerOrgId: order.providerOrgId,
      asset: "CKB",
      amount: "12.5",
    }),
  });

  if (!response.ok) {
    throw new Error(`create invoice failed: ${response.status}`);
  }

  const payload = await response.json();
  if (!payload.invoice) {
    throw new Error("create invoice failed: missing invoice");
  }
}

async function syncSettledFeed() {
  const response = await fetch(`${SETTLEMENT_BASE_URL}/v1/settled-feed`, {
    headers: {
      Accept: "application/json",
      [SERVICE_TOKEN_HEADER]: SETTLEMENT_SERVICE_TOKEN,
    },
  });

  if (!response.ok) {
    throw new Error(`sync settled feed failed: ${response.status}`);
  }
}

async function listRFQs(token) {
  const response = await fetch(`${API_BASE_URL}/api/v1/rfqs`, {
    headers: {
      Accept: "application/json",
      Authorization: `Bearer ${token}`,
    },
  });
  if (!response.ok) throw new Error(`list rfqs failed: ${response.status}`);
  const payload = await response.json();
  return Array.isArray(payload.rfqs) ? payload.rfqs : [];
}

async function listBids(token, rfqId) {
  const response = await fetch(`${API_BASE_URL}/api/v1/rfqs/${rfqId}/bids`, {
    headers: {
      Accept: "application/json",
      Authorization: `Bearer ${token}`,
    },
  });
  if (!response.ok) throw new Error(`list bids failed: ${response.status}`);
  const payload = await response.json();
  return Array.isArray(payload.bids) ? payload.bids : [];
}

async function listOrders(token) {
  const response = await fetch(`${API_BASE_URL}/api/v1/orders`, {
    headers: {
      Accept: "application/json",
      Authorization: `Bearer ${token}`,
    },
  });
  if (!response.ok) throw new Error(`list orders failed: ${response.status}`);
  const payload = await response.json();
  return Array.isArray(payload.orders) ? payload.orders : [];
}

async function getOrderById(token, orderId) {
  const orders = await listOrders(token);
  return orders.find((candidate) => candidate.id === orderId) ?? null;
}

async function listFundingRecords(token) {
  const response = await fetch(`${SETTLEMENT_BASE_URL}/v1/funding-records`, {
    headers: {
      Accept: "application/json",
      Authorization: `Bearer ${token}`,
    },
  });
  if (!response.ok) throw new Error(`list funding records failed: ${response.status}`);
  const payload = await response.json();
  return Array.isArray(payload.records) ? payload.records : [];
}

async function goto(page, url) {
  await page.goto(url, { waitUntil: "networkidle" });
  await waitForHydration(page);
}

async function waitForHydration(page) {
  await page.waitForLoadState("domcontentloaded");
  await page.waitForTimeout(500);
}

async function waitForText(page, text) {
  await page.getByText(text, { exact: false }).first().waitFor({ state: "visible", timeout: 20000 });
}

async function capture(page, entry) {
  const fileName = `${String(entry.order).padStart(2, "0")}-${entry.slug}.png`;
  const relativePath = path.join(entry.businessLine, entry.device, fileName);
  const absolutePath = path.join(OUTPUT_DIR, relativePath);
  await fs.mkdir(path.dirname(absolutePath), { recursive: true });
  await page.screenshot({ path: absolutePath });
  manifest.push({ ...entry, path: relativePath.replaceAll(path.sep, "/") });
  console.log(`captured ${relativePath}`);
}

async function poll(fn, timeoutMs, label) {
  const startedAt = Date.now();
  while (Date.now() - startedAt < timeoutMs) {
    if (await fn()) return;
    await new Promise((resolve) => setTimeout(resolve, 1000));
  }
  throw new Error(`timed out: ${label}`);
}

function datetimeLocalPlusDays(days) {
  const value = new Date(Date.now() + days * 24 * 60 * 60 * 1000);
  value.setSeconds(0, 0);
  const year = value.getFullYear();
  const month = String(value.getMonth() + 1).padStart(2, "0");
  const day = String(value.getDate()).padStart(2, "0");
  const hours = String(value.getHours()).padStart(2, "0");
  const minutes = String(value.getMinutes()).padStart(2, "0");
  return `${year}-${month}-${day}T${hours}:${minutes}`;
}

function trimTrailingSlash(value) {
  return value.replace(/\/$/, "");
}

function capitalize(value) {
  return value.charAt(0).toUpperCase() + value.slice(1);
}

function renderLocalComment(entries) {
  const businessOrder = [
    ["home", "Home"],
    ["buyer", "Buyer"],
    ["provider", "Provider"],
  ];
  const deviceOrder = [
    ["desktop", "Desktop"],
    ["mobile", "Mobile"],
  ];
  const lines = [
    "# E2E Marketplace Screenshots",
    "",
  ];

  for (const [businessKey, businessLabel] of businessOrder) {
    lines.push(`## ${businessLabel}`, "");
    for (const [deviceKey, deviceLabel] of deviceOrder) {
      const rows = entries
        .filter((entry) => entry.businessLine === businessKey && entry.device === deviceKey)
        .sort((left, right) => left.order - right.order);
      if (rows.length === 0) continue;
      lines.push(`### ${deviceLabel}`, "");
      for (const row of rows) {
        lines.push(`- ${row.title}: \`${row.path}\``);
      }
      lines.push("");
    }
  }

  return `${lines.join("\n").trim()}\n`;
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
