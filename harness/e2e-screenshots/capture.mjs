import fs from "node:fs/promises";
import path from "node:path";
import { chromium, devices } from "playwright";

import {
  buildCaptureRunGroups,
  buildFullPagesPlan,
  renderLocalComment,
  resolveCaptureMode,
} from "./catalog.mjs";
import { buildScenarioPreset } from "./scenario.mjs";

const OUTPUT_DIR = process.env.SCREENSHOT_OUTPUT_DIR || "/artifacts";
const WEB_BASE_URL = trimTrailingSlash(process.env.SCREENSHOT_WEB_BASE_URL || "http://web:3000");
const API_BASE_URL = trimTrailingSlash(process.env.SCREENSHOT_API_BASE_URL || "http://api-gateway:8080");
const IAM_BASE_URL = trimTrailingSlash(process.env.SCREENSHOT_IAM_BASE_URL || "http://iam:8081");
const SETTLEMENT_BASE_URL = trimTrailingSlash(process.env.SCREENSHOT_SETTLEMENT_BASE_URL || "http://settlement:8083");
const SETTLEMENT_SERVICE_TOKEN = process.env.SCREENSHOT_SETTLEMENT_SERVICE_TOKEN || "local-settlement-service-token";
const SETTLEMENT_INVOICE_ASSET = process.env.SCREENSHOT_INVOICE_ASSET || "CKB";
const SETTLEMENT_INVOICE_AMOUNT = process.env.SCREENSHOT_INVOICE_AMOUNT || "12.5";
const EXECUTION_BASE_URL = trimTrailingSlash(process.env.SCREENSHOT_EXECUTION_BASE_URL || "http://execution:8085");
const EXECUTION_EVENT_TOKEN = process.env.SCREENSHOT_EXECUTION_EVENT_TOKEN || "local-execution-event-token";
const CAPTURE_MODE = resolveCaptureMode(process.env.SCREENSHOT_CAPTURE_MODE);
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
    const runGroups = buildCaptureRunGroups({
      mode: CAPTURE_MODE,
      deviceKeys: DEVICE_PRESETS.map((device) => device.key),
    });

    for (const runGroup of runGroups) {
      const groupDevices = DEVICE_PRESETS.filter((device) => runGroup.deviceKeys.includes(device.key));

      if (CAPTURE_MODE === "full-pages") {
        const seededScenario = await seedFullPagesScenario(browser, runGroup.seedKey, groupDevices[0]);
        for (const device of groupDevices) {
          await captureFullPagesForDevice(browser, device, seededScenario);
        }
      } else {
        for (const device of groupDevices) {
          await runJourneyForDevice(browser, device);
        }
      }
    }

    await fs.writeFile(path.join(OUTPUT_DIR, "manifest.json"), JSON.stringify(manifest, null, 2) + "\n", "utf8");
    await fs.writeFile(path.join(OUTPUT_DIR, "comment.md"), renderLocalComment(manifest), "utf8");
    console.log(`screenshots saved to ${OUTPUT_DIR}`);
  } finally {
    await browser.close();
  }
}

async function runJourneyForDevice(browser, device) {
  const suffix = `${device.key}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
  const scenario = buildScenarioPreset(device.key);
  const buyer = await createUser("buyer", suffix);
  const provider = await createUser("provider", suffix);
  const requestTitle = scenario.requestTitle;

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
    await createRequest(buyerPage, requestTitle, scenario.requestBudgetDollars);
    const requestRecord = await waitForBuyerRequest(buyer.token, buyer.organizationId, requestTitle);
    const rfqId = requestRecord.id;
    await waitForText(buyerPage, requestTitle);
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

    await goto(providerPage, `${WEB_BASE_URL}/provider/rfqs/${rfqId}`);
    await waitForText(providerPage, requestTitle);
    await capture(providerPage, {
      businessLine: "provider",
      device: device.key,
      order: 2,
      slug: "open-request",
      title: "Provider open request",
    });
    await providerPage.locator('input[name="quoteDollars"]').fill(scenario.quoteDollars);
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
    await waitForHydration(providerPage);
    await waitForRequestBids(buyer.token, rfqId, 1);

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
    const orderId = await waitForAwardedOrderId(buyer.token, rfqId);

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

async function seedFullPagesScenario(browser, seedKey, device) {
  const suffix = `${seedKey}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
  const scenario = buildScenarioPreset(seedKey);
  const buyer = await createUser("buyer", suffix);
  const provider = await createUser("provider", suffix);
  const ops = await createUser("ops", suffix);
  const requestTitle = scenario.requestTitle;
  const buyerContext = await browser.newContext(device.contextOptions);
  const buyerPage = await buyerContext.newPage();
  const providerContext = await browser.newContext(device.contextOptions);
  const providerPage = await providerContext.newPage();

  try {
    await goto(buyerPage, `${WEB_BASE_URL}/login?next=/buyer`);
    await login(buyerPage, buyer.email, buyer.password, /\/buyer$/);
    await goto(buyerPage, `${WEB_BASE_URL}/buyer/rfqs/create`);
    await createRequest(buyerPage, requestTitle, scenario.requestBudgetDollars);
    const requestRecord = await waitForBuyerRequest(buyer.token, buyer.organizationId, requestTitle);
    const rfqId = requestRecord.id;

    await goto(providerPage, `${WEB_BASE_URL}/login?next=/provider`);
    await login(providerPage, provider.email, provider.password, /\/provider$/);

    await goto(providerPage, `${WEB_BASE_URL}/provider/rfqs/${rfqId}`);
    await waitForText(providerPage, requestTitle);
    await providerPage.locator('input[name="quoteDollars"]').fill(scenario.quoteDollars);
    await Promise.all([
      providerPage.waitForURL(/\/provider$/),
      providerPage.getByRole("button", { name: /submit proposal/i }).click(),
    ]);
    await waitForHydration(providerPage);
    await waitForRequestBids(buyer.token, rfqId, 1);

    await goto(buyerPage, `${WEB_BASE_URL}/buyer`);
    await waitForText(buyerPage, requestTitle);
    await clickAwardForRequest(buyerPage, requestTitle);
    const orderId = await waitForAwardedOrderId(buyer.token, rfqId);

    const settledOrder = await settleOrderFlow(provider.token, orderId);
    await createInvoice(settledOrder);
    await syncSettledFeed();
    await waitForSettledInvoice(provider.token, orderId);

    return { buyer, provider, ops, requestTitle, rfqId, orderId };
  } finally {
    await buyerContext.close();
    await providerContext.close();
  }
}

async function captureFullPagesForDevice(browser, device, seededScenario) {
  const entries = buildFullPagesEntryMap(device.key, {
    rfqId: seededScenario.rfqId,
    orderId: seededScenario.orderId,
  });

  const publicContext = await browser.newContext(device.contextOptions);
  const publicPage = await publicContext.newPage();
  const buyerContext = await browser.newContext(device.contextOptions);
  const buyerPage = await buyerContext.newPage();
  const providerContext = await browser.newContext(device.contextOptions);
  const providerPage = await providerContext.newPage();
  const opsContext = await browser.newContext(device.contextOptions);
  const opsPage = await opsContext.newPage();

  try {
    await goto(publicPage, `${WEB_BASE_URL}${getEntry(entries, "public", "home").url}`);
    await capture(publicPage, getEntry(entries, "public", "home"), { fullPage: true });

    await goto(publicPage, `${WEB_BASE_URL}${getEntry(entries, "public", "buyer-login").url}`);
    await capture(publicPage, getEntry(entries, "public", "buyer-login"), { fullPage: true });

    await goto(publicPage, `${WEB_BASE_URL}${getEntry(entries, "public", "provider-login").url}`);
    await capture(publicPage, getEntry(entries, "public", "provider-login"), { fullPage: true });

    await goto(publicPage, `${WEB_BASE_URL}${getEntry(entries, "public", "internal-login").url}`);
    await capture(publicPage, getEntry(entries, "public", "internal-login"), { fullPage: true });

    await goto(buyerPage, `${WEB_BASE_URL}/login?next=/buyer`);
    await login(buyerPage, seededScenario.buyer.email, seededScenario.buyer.password, /\/buyer$/);
    await goto(buyerPage, `${WEB_BASE_URL}${getEntry(entries, "buyer", "dashboard").url}`);
    await capture(buyerPage, getEntry(entries, "buyer", "dashboard"), { fullPage: true });

    await goto(buyerPage, `${WEB_BASE_URL}${getEntry(entries, "buyer", "post-request").url}`);
    await capture(buyerPage, getEntry(entries, "buyer", "post-request"), { fullPage: true });

    await goto(buyerPage, `${WEB_BASE_URL}${getEntry(entries, "buyer", "order-detail").url}`);
    await capture(buyerPage, getEntry(entries, "buyer", "order-detail"), { fullPage: true });

    await goto(providerPage, `${WEB_BASE_URL}/login?next=/provider`);
    await login(providerPage, seededScenario.provider.email, seededScenario.provider.password, /\/provider$/);
    await goto(providerPage, `${WEB_BASE_URL}${getEntry(entries, "provider", "dashboard").url}`);
    await capture(providerPage, getEntry(entries, "provider", "dashboard"), { fullPage: true });

    await goto(providerPage, `${WEB_BASE_URL}${getEntry(entries, "provider", "rfqs").url}`);
    await capture(providerPage, getEntry(entries, "provider", "rfqs"), { fullPage: true });

    await goto(providerPage, `${WEB_BASE_URL}${getEntry(entries, "provider", "rfq-detail").url}`);
    await capture(providerPage, getEntry(entries, "provider", "rfq-detail"), { fullPage: true });

    await goto(providerPage, `${WEB_BASE_URL}${getEntry(entries, "provider", "proposals").url}`);
    await capture(providerPage, getEntry(entries, "provider", "proposals"), { fullPage: true });

    await goto(providerPage, `${WEB_BASE_URL}${getEntry(entries, "provider", "order-detail").url}`);
    await capture(providerPage, getEntry(entries, "provider", "order-detail"), { fullPage: true });

    await goto(opsPage, `${WEB_BASE_URL}/internal/login?next=/ops`);
    await login(opsPage, seededScenario.ops.email, seededScenario.ops.password, /\/ops$/);
    await goto(opsPage, `${WEB_BASE_URL}${getEntry(entries, "ops", "dashboard").url}`);
    await capture(opsPage, getEntry(entries, "ops", "dashboard"), { fullPage: true });

    await goto(opsPage, `${WEB_BASE_URL}${getEntry(entries, "ops", "applications").url}`);
    await capture(opsPage, getEntry(entries, "ops", "applications"), { fullPage: true });

    await goto(opsPage, `${WEB_BASE_URL}${getEntry(entries, "ops", "application-detail").url}`);
    await capture(opsPage, getEntry(entries, "ops", "application-detail"), { fullPage: true });

    await goto(opsPage, `${WEB_BASE_URL}${getEntry(entries, "ops", "disputes").url}`);
    await capture(opsPage, getEntry(entries, "ops", "disputes"), { fullPage: true });
  } finally {
    await publicContext.close();
    await buyerContext.close();
    await providerContext.close();
    await opsContext.close();
  }
}

function buildFullPagesEntryMap(device, ids = {}) {
  return new Map(
    buildFullPagesPlan({
      device,
      rfqId: ids.rfqId ?? "rfq_pending",
      orderId: ids.orderId ?? "ord_pending",
      applicationId: ids.applicationId ?? "app_1",
    }).map((entry) => [`${entry.businessLine}:${entry.slug}`, entry]),
  );
}

function getEntry(entries, businessLine, slug) {
  const entry = entries.get(`${businessLine}:${slug}`);
  if (!entry) {
    throw new Error(`missing capture entry for ${businessLine}:${slug}`);
  }
  return entry;
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
  return {
    email,
    password: PASSWORD,
    token: payload.session?.token ?? "",
    organizationId: payload.organization?.id ?? "",
  };
}

async function login(page, email, password, destinationPattern = /\/(buyer|provider)$/) {
  await page.locator('input[name="email"]').fill(email);
  await page.locator('input[name="password"]').fill(password);
  await Promise.all([
    page.waitForURL(destinationPattern),
    page.locator('form[action="/auth/login"] button[type="submit"]').click(),
  ]);
  await waitForHydration(page);
}

async function createRequest(page, title, budgetDollars) {
  await page.locator('input[name="title"]').fill(title);
  await page.locator('input[name="budgetDollars"]').fill(budgetDollars);
  await page.locator('input[name="responseDeadlineAt"]').fill(datetimeLocalPlusDays(5));
  await Promise.all([
    page.waitForURL(/\/buyer$/),
    page.getByRole("button", { name: /^post request$/i }).click(),
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

async function waitForBuyerRequest(token, buyerOrgId, title) {
  let requestRecord = null;

  await poll(async () => {
    const rfqs = await listRFQs(token);
    const matches = rfqs
      .filter((candidate) => candidate.buyerOrgId === buyerOrgId && candidate.title === title)
      .sort((left, right) => Date.parse(right.createdAt ?? "") - Date.parse(left.createdAt ?? ""));

    requestRecord = matches[0] ?? null;
    return Boolean(requestRecord?.id);
  }, 25000, `wait for request ${title}`);

  return requestRecord;
}

async function waitForRequestBids(token, rfqId, minCount) {
  await poll(async () => {
    const bids = await listBids(token, rfqId);
    return bids.length >= minCount;
  }, 25000, `wait for bids on ${rfqId}`);
}

async function waitForAwardedOrderId(token, rfqId) {
  let awardedOrderId = "";

  await poll(async () => {
    const rfqs = await listRFQs(token);
    const item = rfqs.find((candidate) => candidate.id === rfqId);
    if (!item?.orderId) return false;
    awardedOrderId = item.orderId;
    return true;
  }, 25000, `wait for awarded order on ${rfqId}`);

  return awardedOrderId;
}

async function settleOrderFlow(token, orderId) {
  await settleOrderMilestone(token, orderId);

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

async function settleOrderMilestone(token, orderId) {
  const order = await getOrderById(token, orderId);
  const milestoneId = order?.milestones?.[0]?.id || "ms_1";

  const response = await fetch(`${EXECUTION_BASE_URL}/v1/carrier/events`, {
    method: "POST",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
      [SERVICE_TOKEN_HEADER]: EXECUTION_EVENT_TOKEN,
    },
    body: JSON.stringify({
      orderId,
      milestoneId,
      eventType: "milestone_ready",
      summary: "Screenshot harness completed the mocked provider step.",
    }),
  });

  if (!response.ok) {
    throw new Error(`settle order milestone failed: ${response.status}`);
  }
}

async function createInvoice(order) {
  const milestoneId = order.milestones[0]?.id || "ms_1";

  const response = await fetch(`${SETTLEMENT_BASE_URL}/v1/invoices`, {
    method: "POST",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
      [SERVICE_TOKEN_HEADER]: SETTLEMENT_SERVICE_TOKEN,
    },
    body: JSON.stringify({
      orderId: order.id,
      milestoneId,
      buyerOrgId: order.buyerOrgId,
      providerOrgId: order.providerOrgId,
      asset: SETTLEMENT_INVOICE_ASSET,
      amount: SETTLEMENT_INVOICE_AMOUNT,
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
  await page.goto(url, { waitUntil: "load" });
  await waitForHydration(page);
}

async function waitForHydration(page) {
  await page.waitForLoadState("domcontentloaded");
  await page.waitForTimeout(500);
}

async function waitForText(page, text) {
  await page.getByText(text, { exact: false }).first().waitFor({ state: "visible", timeout: 20000 });
}

async function capture(page, entry, options = {}) {
  const fileName = `${String(entry.order).padStart(2, "0")}-${entry.slug}.png`;
  const relativePath = path.join(entry.businessLine, entry.device, fileName);
  const absolutePath = path.join(OUTPUT_DIR, relativePath);
  await fs.mkdir(path.dirname(absolutePath), { recursive: true });
  await page.screenshot({ path: absolutePath, ...(options.fullPage ? { fullPage: true } : {}) });
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

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
