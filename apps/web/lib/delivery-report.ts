export type DeliveryReportBlock =
  | {
      type: "paragraph";
      text: string;
    }
  | {
      type: "list";
      items: string[];
    };

export interface DeliveryReportSection {
  title: string;
  blocks: DeliveryReportBlock[];
}

export type ParsedDeliveryReport =
  | {
      kind: "empty";
      sections: [];
      path?: undefined;
      message?: undefined;
    }
  | {
      kind: "receipt";
      sections: [];
      path: string;
      message: string;
    }
  | {
      kind: "report";
      sections: DeliveryReportSection[];
      path?: undefined;
      message?: undefined;
    };

const receiptPattern = /^Carrier execution completed\.\s+Result saved to\s+(.+)$/i;
const headingPattern = /^\s{0,3}#{1,3}\s+(.+?)\s*$/;
const bulletPattern = /^\s*[-*]\s+(.+?)\s*$/;

export function parseDeliveryReport(summary: string | null | undefined): ParsedDeliveryReport {
  const trimmed = summary?.trim() ?? "";
  if (!trimmed) {
    return { kind: "empty", sections: [] };
  }

  const receiptMatch = trimmed.match(receiptPattern);
  if (receiptMatch) {
    return {
      kind: "receipt",
      sections: [],
      path: receiptMatch[1]?.trim() ?? "",
      message: trimmed,
    };
  }

  const lines = trimmed.split(/\r?\n/);
  const sections: DeliveryReportSection[] = [];
  let currentTitle = "Report";
  let currentBody: string[] = [];
  let seenHeading = false;

  const pushSection = () => {
    const blocks = parseSectionBlocks(currentBody);
    if (!seenHeading && blocks.length === 0) {
      return;
    }
    sections.push({
      title: currentTitle,
      blocks,
    });
  };

  for (const rawLine of lines) {
    const headingMatch = rawLine.match(headingPattern);
    if (headingMatch) {
      if (seenHeading || currentBody.some((line) => line.trim() !== "")) {
        pushSection();
      }
      currentTitle = headingMatch[1]!.trim();
      currentBody = [];
      seenHeading = true;
      continue;
    }
    currentBody.push(rawLine);
  }

  if (seenHeading || currentBody.some((line) => line.trim() !== "")) {
    pushSection();
  }

  if (sections.length === 0) {
    return {
      kind: "report",
      sections: [
        {
          title: "Report",
          blocks: parseSectionBlocks(lines),
        },
      ],
    };
  }

  return { kind: "report", sections };
}

function parseSectionBlocks(lines: string[]): DeliveryReportBlock[] {
  const blocks: DeliveryReportBlock[] = [];
  let paragraphLines: string[] = [];
  let bulletItems: string[] = [];

  const flushParagraph = () => {
    if (paragraphLines.length === 0) return;
    blocks.push({
      type: "paragraph",
      text: paragraphLines.join(" ").trim(),
    });
    paragraphLines = [];
  };

  const flushBullets = () => {
    if (bulletItems.length === 0) return;
    blocks.push({
      type: "list",
      items: [...bulletItems],
    });
    bulletItems = [];
  };

  for (const rawLine of lines) {
    const line = rawLine.trim();
    if (!line) {
      flushParagraph();
      flushBullets();
      continue;
    }

    const bulletMatch = line.match(bulletPattern);
    if (bulletMatch) {
      flushParagraph();
      bulletItems.push(bulletMatch[1]!.trim());
      continue;
    }

    flushBullets();
    paragraphLines.push(line);
  }

  flushParagraph();
  flushBullets();

  return blocks;
}
