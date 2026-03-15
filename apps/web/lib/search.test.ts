import { describe, expect, it } from "bun:test";
import { buildSearchQuery, parseSearchQuery } from "./search";

describe("search utilities", () => {
  it("builds query with all params", () => {
    const query = buildSearchQuery({
      q: "gpu",
      category: "compute",
      tags: ["fast", "cheap"],
      minPrice: 1000,
      maxPrice: 5000,
      providerOrgId: "org_1",
    });
    expect(query.get("q")).toBe("gpu");
    expect(query.get("category")).toBe("compute");
    expect(query.getAll("tag")).toEqual(["fast", "cheap"]);
    expect(query.get("minPrice")).toBe("1000");
    expect(query.get("maxPrice")).toBe("5000");
    expect(query.get("providerOrgId")).toBe("org_1");
  });

  it("builds empty query for no params", () => {
    const query = buildSearchQuery({});
    expect(query.toString()).toBe("");
  });

  it("round-trips through parse", () => {
    const original = {
      q: "agent",
      category: "ai",
      tags: ["gpu"],
      minPrice: 500,
      maxPrice: 10000,
    };
    const query = buildSearchQuery(original);
    const parsed = parseSearchQuery(query);
    expect(parsed.q).toBe("agent");
    expect(parsed.category).toBe("ai");
    expect(parsed.tags).toEqual(["gpu"]);
    expect(parsed.minPrice).toBe(500);
    expect(parsed.maxPrice).toBe(10000);
  });

  it("parses empty query", () => {
    const parsed = parseSearchQuery(new URLSearchParams());
    expect(parsed).toEqual({});
  });
});
