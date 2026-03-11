import { describe, expect, it } from "bun:test";

import {
  formatMoney,
  fundingModes,
  milestoneStates,
  orderStatuses,
  usageChargeKinds,
} from "./index";

describe("contracts", () => {
  it("keeps canonical enum values aligned for marketplace flows", () => {
    expect(fundingModes).toEqual(["prepaid", "credit"]);
    expect(orderStatuses).toContain("awaiting_budget");
    expect(milestoneStates).toContain("settled");
    expect(usageChargeKinds).toContain("external_api");
  });

  it("formats cents for dashboard summaries", () => {
    expect(formatMoney(123456)).toBe("$1,234.56");
  });
});
