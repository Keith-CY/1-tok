import assert from "node:assert/strict";
import test from "node:test";

import { buildScenarioPreset } from "./scenario.mjs";

test("buildScenarioPreset uses human-readable marketplace titles", () => {
  const shared = buildScenarioPreset("shared");
  const desktop = buildScenarioPreset("desktop");

  assert.equal(shared.requestTitle, "Carrier dispute triage package");
  assert.equal(desktop.requestTitle, "Warehouse relaunch pricing sprint");
  assert.equal(shared.quoteDollars, "2410.00");
  assert.equal(shared.requestTitle.includes("shared-"), false);
  assert.equal(desktop.requestTitle.includes("desktop-"), false);
});
