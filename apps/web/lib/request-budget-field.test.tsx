import { describe, expect, it } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { RequestBudgetField, dollarsInputToCents } from "@/components/request-budget-field";

describe("request budget field", () => {
  it("converts a dollar input string into cents", () => {
    expect(dollarsInputToCents("680")).toBe("68000");
    expect(dollarsInputToCents("680.55")).toBe("68055");
    expect(dollarsInputToCents("0")).toBe("");
    expect(dollarsInputToCents("")).toBe("");
  });

  it("renders a hidden budgetCents field and usd copy", () => {
    const html = renderToStaticMarkup(<RequestBudgetField name="budgetCents" />);

    expect(html).toContain('name="budgetCents"');
    expect(html).toContain('type="hidden"');
    expect(html).toContain("Budget");
    expect(html).toContain("USD");
  });
});
