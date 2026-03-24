import { describe, expect, it } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { RequestDeadlineField, combineDeadlineValue } from "@/components/request-deadline-field";

describe("request deadline field", () => {
  it("combines date and time into a local datetime field value", () => {
    expect(combineDeadlineValue("2026-03-26", "14:30")).toBe("2026-03-26T14:30");
    expect(combineDeadlineValue("2026-03-26", "")).toBe("");
  });

  it("renders a hidden responseDeadlineAt field instead of a native datetime-local input", () => {
    const html = renderToStaticMarkup(<RequestDeadlineField name="responseDeadlineAt" />);

    expect(html).toContain('name="responseDeadlineAt"');
    expect(html).toContain('type="hidden"');
    expect(html).not.toContain('type="datetime-local"');
    expect(html).toContain("Delivery window");
    expect(html).toContain("required");
  });
});
