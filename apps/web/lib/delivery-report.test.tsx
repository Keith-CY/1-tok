import { describe, expect, it } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { DeliveryReportPanel } from "@/components/delivery-report-panel";
import { parseDeliveryReport } from "@/lib/delivery-report";

describe("delivery report", () => {
  it("treats legacy carrier receipts as execution receipts", () => {
    const parsed = parseDeliveryReport("Carrier execution completed. Result saved to /workspace/1tok/ord_11/ms_1/result.md");

    expect(parsed.kind).toBe("receipt");
    expect(parsed.path).toBe("/workspace/1tok/ord_11/ms_1/result.md");
  });

  it("parses markdown reports into titled sections", () => {
    const parsed = parseDeliveryReport(`
# Summary
Three vendors fit the brief.

## Findings
- Vendor A is fastest to onboard.
- Vendor B is the cheapest.

## Recommendation
Start with Vendor A and keep Vendor B in reserve.
`);

    expect(parsed.kind).toBe("report");
    expect(parsed.sections.map((section) => section.title)).toEqual(["Summary", "Findings", "Recommendation"]);
  });

  it("renders report sections instead of a raw preformatted blob", () => {
    const html = renderToStaticMarkup(
      <DeliveryReportPanel
        summary={`
# Summary
Three vendors fit the brief.

## Findings
- Vendor A is fastest to onboard.

## Recommendation
Start with Vendor A.
`}
      />,
    );

    expect(html).toContain("Delivery report");
    expect(html).toContain("Summary");
    expect(html).toContain("Findings");
    expect(html).toContain("<li>Vendor A is fastest to onboard.</li>");
    expect(html).not.toContain("whitespace-pre-wrap");
  });
});
