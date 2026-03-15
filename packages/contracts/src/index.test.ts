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

import { isJobState, isFundingMode, isOrderStatus, isMilestoneState, isUsageChargeKind } from ".";

describe("type guards", () => {
  it("validates funding modes", () => {
    expect(isFundingMode("prepaid")).toBe(true);
    expect(isFundingMode("credit")).toBe(true);
    expect(isFundingMode("invalid")).toBe(false);
    expect(isFundingMode(42)).toBe(false);
  });

  it("validates order statuses", () => {
    expect(isOrderStatus("running")).toBe(true);
    expect(isOrderStatus("completed")).toBe(true);
    expect(isOrderStatus("nope")).toBe(false);
  });

  it("validates milestone states", () => {
    expect(isMilestoneState("pending")).toBe(true);
    expect(isMilestoneState("settled")).toBe(true);
    expect(isMilestoneState("nope")).toBe(false);
  });

  it("validates usage charge kinds", () => {
    expect(isUsageChargeKind("token")).toBe(true);
    expect(isUsageChargeKind("step")).toBe(true);
    expect(isUsageChargeKind("nope")).toBe(false);
  });

  it("validates job states", () => {
    expect(isJobState("pending")).toBe(true);
    expect(isJobState("running")).toBe(true);
    expect(isJobState("completed")).toBe(true);
    expect(isJobState("failed")).toBe(true);
    expect(isJobState("cancelled")).toBe(true);
    expect(isJobState("nope")).toBe(false);
  });
});

import { assertFundingMode, assertOrderStatus, assertMilestoneState, assertUsageChargeKind } from ".";

describe("assertion functions", () => {
  it("passes for valid funding mode", () => {
    expect(() => assertFundingMode("prepaid")).not.toThrow();
  });

  it("throws for invalid funding mode", () => {
    expect(() => assertFundingMode("invalid")).toThrow("Invalid funding mode");
  });

  it("passes for valid order status", () => {
    expect(() => assertOrderStatus("running")).not.toThrow();
  });

  it("throws for invalid order status", () => {
    expect(() => assertOrderStatus("bogus")).toThrow("Invalid order status");
  });

  it("passes for valid milestone state", () => {
    expect(() => assertMilestoneState("settled")).not.toThrow();
  });

  it("throws for invalid milestone state", () => {
    expect(() => assertMilestoneState("bogus")).toThrow("Invalid milestone state");
  });

  it("passes for valid usage charge kind", () => {
    expect(() => assertUsageChargeKind("token")).not.toThrow();
  });

  it("throws for invalid usage charge kind", () => {
    expect(() => assertUsageChargeKind("bogus")).toThrow("Invalid usage charge kind");
  });
});
