import { parseDeliveryReport } from "@/lib/delivery-report";

export function DeliveryReportPanel({ summary }: { summary: string }) {
  const parsed = parseDeliveryReport(summary);
  if (parsed.kind === "empty") {
    return null;
  }

  if (parsed.kind === "receipt") {
    return (
      <div className="mt-4 bg-secondary/40 p-4 md:p-5">
        <div className="text-[11px] uppercase tracking-[0.16em] text-muted-foreground">Execution receipt</div>
        <p className="mt-3 text-sm leading-7 text-muted-foreground">
          The carrier finished execution and stored the report artifact at the path below.
        </p>
        <div className="mt-3 break-all bg-background/70 px-3 py-3 font-mono text-xs text-foreground">{parsed.path}</div>
      </div>
    );
  }

  return (
    <div className="mt-4 bg-secondary/40 p-4 md:p-5">
      <div className="text-[11px] uppercase tracking-[0.16em] text-muted-foreground">Delivery report</div>
      <div className="mt-4 space-y-4">
        {parsed.sections.map((section) => (
          <section key={section.title} className="space-y-3 bg-background/75 px-4 py-4">
            <div className="text-xs font-medium uppercase tracking-[0.16em] text-primary">{section.title}</div>
            <div className="space-y-3">
              {section.blocks.map((block, index) =>
                block.type === "paragraph" ? (
                  <p key={`${section.title}-p-${index}`} className="text-sm leading-7 text-foreground">
                    {block.text}
                  </p>
                ) : (
                  <ul
                    key={`${section.title}-l-${index}`}
                    className="list-disc space-y-2 pl-5 text-sm leading-7 text-foreground marker:text-primary"
                  >
                    {block.items.map((item) => (
                      <li key={`${section.title}-${item}`}>{item}</li>
                    ))}
                  </ul>
                ),
              )}
            </div>
          </section>
        ))}
      </div>
    </div>
  );
}
