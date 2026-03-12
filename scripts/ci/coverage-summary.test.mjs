import { describe, expect, test } from "bun:test";

import {
  parseBunCoverageSummary,
  parseGoCoverageSummary,
  renderCoverageMarkdown,
} from "./coverage-summary.mjs";

describe("coverage summary parsing", () => {
  test("parses Go total coverage from go tool cover output", () => {
    const parsed = parseGoCoverageSummary(`
github.com/chenyu/1-tok/internal/services/settlement/server.go:10:\tServeHTTP\t56.4%
total:\t\t\t\t\t\t\t\t(statements)\t57.9%
`);

    expect(parsed.linesPct).toBe(57.9);
  });

  test("parses Bun coverage totals from stdout table", () => {
    const parsed = parseBunCoverageSummary(`
-----------------------------------------------|---------|---------|-------------------
File                                           | % Funcs | % Lines | Uncovered Line #s
-----------------------------------------------|---------|---------|-------------------
All files                                      |   97.06 |   87.97 |
 lib/api.ts                                    |   98.08 |   99.51 | 303
-----------------------------------------------|---------|---------|-------------------
`);

    expect(parsed.funcsPct).toBe(97.06);
    expect(parsed.linesPct).toBe(87.97);
  });
});

describe("coverage summary rendering", () => {
  test("renders markdown table for go and bun suites", () => {
    const markdown = renderCoverageMarkdown({
      suites: [
        { name: "Go", funcsPct: null, linesPct: 57.9, metricLabel: "Statements" },
        { name: "Web", funcsPct: 97.06, linesPct: 87.97, metricLabel: "Lines" },
        { name: "Contracts", funcsPct: 100, linesPct: 100, metricLabel: "Lines" },
      ],
    });

    expect(markdown).toContain("| Suite | Funcs % | Lines % | Primary Metric |");
    expect(markdown).toContain("| Go | - | 57.90 | Statements |");
    expect(markdown).toContain("| Web | 97.06 | 87.97 | Lines |");
    expect(markdown).toContain("| Contracts | 100.00 | 100.00 | Lines |");
  });
});
