import assert from "node:assert/strict";
import test from "node:test";

import {
  buildCaptureRunGroups,
  buildFullPagesPlan,
  renderGitHubPRComment,
  renderLocalComment,
  resolveCaptureMode,
} from "./catalog.mjs";

test("resolveCaptureMode defaults to journey", () => {
  assert.equal(resolveCaptureMode(undefined), "journey");
  assert.equal(resolveCaptureMode(""), "journey");
});

test("resolveCaptureMode accepts full-pages", () => {
  assert.equal(resolveCaptureMode("full-pages"), "full-pages");
});

test("resolveCaptureMode rejects unknown capture modes", () => {
  assert.throws(() => resolveCaptureMode("unknown"), /unknown screenshot capture mode/i);
});

test("buildCaptureRunGroups reuses one seed across devices for full-pages", () => {
  assert.deepEqual(
    buildCaptureRunGroups({
      mode: "full-pages",
      deviceKeys: ["desktop", "mobile"],
    }),
    [
      { seedKey: "shared", deviceKeys: ["desktop", "mobile"] },
    ],
  );
});

test("buildCaptureRunGroups keeps journey runs isolated per device", () => {
  assert.deepEqual(
    buildCaptureRunGroups({
      mode: "journey",
      deviceKeys: ["desktop", "mobile"],
    }),
    [
      { seedKey: "desktop", deviceKeys: ["desktop"] },
      { seedKey: "mobile", deviceKeys: ["mobile"] },
    ],
  );
});

test("buildFullPagesPlan includes only distinct pages with live ids wired in", () => {
  const plan = buildFullPagesPlan({
    device: "desktop",
    rfqId: "rfq_live_1",
    orderId: "ord_live_1",
    applicationId: "app_1",
  });

  assert.deepEqual(
    plan.map((entry) => [entry.businessLine, entry.slug, entry.url]),
    [
      ["public", "home", "/"],
      ["public", "buyer-login", "/login?next=/buyer"],
      ["public", "provider-login", "/login?next=/provider"],
      ["public", "internal-login", "/internal/login?next=/ops"],
      ["buyer", "dashboard", "/buyer"],
      ["buyer", "post-request", "/buyer/rfqs/create"],
      ["buyer", "order-detail", "/buyer/orders/ord_live_1"],
      ["provider", "dashboard", "/provider"],
      ["provider", "rfqs", "/provider/rfqs"],
      ["provider", "rfq-detail", "/provider/rfqs/rfq_live_1"],
      ["provider", "proposals", "/provider/proposals"],
      ["provider", "order-detail", "/provider/orders/ord_live_1"],
      ["ops", "dashboard", "/ops"],
      ["ops", "applications", "/ops/applications"],
      ["ops", "application-detail", "/ops/applications/app_1"],
      ["ops", "disputes", "/ops/disputes"],
    ],
  );

  assert.equal(plan.every((entry) => !entry.url.includes("/buyer/leaderboard")), true);
  assert.equal(plan.every((entry) => !entry.url.includes("/buyer/listings")), true);
  assert.equal(plan.every((entry) => !entry.url.includes("/provider/carrier")), true);
  assert.equal(plan.every((entry) => !entry.url.includes("/provider/listings")), true);
});

test("renderLocalComment includes ops entries for full-pages mode", () => {
  const comment = renderLocalComment([
    { businessLine: "public", device: "desktop", order: 1, slug: "home", title: "Home", path: "public/desktop/01-home.png" },
    { businessLine: "ops", device: "desktop", order: 12, slug: "dashboard", title: "Ops dashboard", path: "ops/desktop/12-dashboard.png" },
  ]);

  assert.match(comment, /## Public/);
  assert.match(comment, /## Operations/);
  assert.match(comment, /Ops dashboard/);
});

test("renderGitHubPRComment reuses manifest entries to build image tables", () => {
  const comment = renderGitHubPRComment(
    [
      { businessLine: "buyer", device: "desktop", order: 5, slug: "dashboard", title: "Buyer dashboard", path: "buyer/desktop/05-dashboard.png" },
      { businessLine: "buyer", device: "desktop", order: 7, slug: "order-detail", title: "Buyer order detail", path: "buyer/desktop/07-order-detail.png" },
      { businessLine: "ops", device: "mobile", order: 13, slug: "dashboard", title: "Ops dashboard", path: "ops/mobile/13-dashboard.png" },
    ],
    {
      repository: "acme/1-tok",
      refName: "screenshots/pr-42",
      runUrl: "https://github.com/acme/1-tok/actions/runs/123456",
    },
  );

  assert.match(comment, /<!-- 1-tok-e2e-screenshots -->/);
  assert.match(comment, /- Run: https:\/\/github\.com\/acme\/1-tok\/actions\/runs\/123456/);
  assert.match(comment, /### Buyer/);
  assert.match(comment, /### Operations/);
  assert.match(comment, /\| Buyer dashboard \| Buyer order detail \|/);
  assert.match(comment, /!\[dashboard\]\(https:\/\/raw\.githubusercontent\.com\/acme\/1-tok\/screenshots\/pr-42\/buyer\/desktop\/05-dashboard\.png\)/);
  assert.match(comment, /!\[dashboard\]\(https:\/\/raw\.githubusercontent\.com\/acme\/1-tok\/screenshots\/pr-42\/ops\/mobile\/13-dashboard\.png\)/);
});
