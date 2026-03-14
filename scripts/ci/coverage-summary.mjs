import { mkdir, readFile, writeFile } from "node:fs/promises";
import { dirname } from "node:path";

function parsePercent(raw) {
  const normalized = raw?.trim();
  if (!normalized || normalized === "-") {
    return null;
  }

  const value = Number.parseFloat(normalized.replace(/%$/, ""));
  return Number.isFinite(value) ? value : null;
}

export function parseGoCoverageSummary(text) {
  const match = text.match(/total:\s+\(statements\)\s+([0-9.]+)%/i);
  if (!match) {
    throw new Error("could not find Go coverage total");
  }

  // Parse per-function lines to compute function coverage percentage.
  // Each non-total line from `go tool cover -func` looks like:
  //   package/file.go:line:  FuncName    XX.X%
  const funcLines = text.split("\n").filter(
    (line) => line.includes("%") && !line.startsWith("total:"),
  );
  let coveredFuncs = 0;
  let totalFuncs = 0;
  for (const line of funcLines) {
    const pctMatch = line.match(/([0-9.]+)%\s*$/);
    if (pctMatch) {
      totalFuncs += 1;
      if (Number.parseFloat(pctMatch[1]) > 0) {
        coveredFuncs += 1;
      }
    }
  }
  const funcsPct = totalFuncs > 0 ? (coveredFuncs / totalFuncs) * 100 : null;

  return {
    funcsPct,
    linesPct: parsePercent(match[1]),
    metricLabel: "Statements",
  };
}

export function parseBunCoverageSummary(text) {
  const match = text.match(/All files\s+\|\s+([0-9.-]+)\s+\|\s+([0-9.-]+)\s+\|/i);
  if (!match) {
    throw new Error("could not find Bun coverage totals");
  }

  return {
    funcsPct: parsePercent(match[1]),
    linesPct: parsePercent(match[2]),
    metricLabel: "Lines",
  };
}

function formatPercent(value) {
  return value == null ? "-" : value.toFixed(2);
}

export function renderCoverageMarkdown({ suites }) {
  const lines = [
    "## Coverage",
    "",
    "| Suite | Funcs % | Lines % | Primary Metric |",
    "| --- | ---: | ---: | --- |",
  ];

  for (const suite of suites) {
    lines.push(
      `| ${suite.name} | ${formatPercent(suite.funcsPct)} | ${formatPercent(suite.linesPct)} | ${suite.metricLabel} |`,
    );
  }

  return `${lines.join("\n")}\n`;
}

async function readText(path) {
  return readFile(path, "utf8");
}

async function loadSuite({ name, path, parser, fallbackMetricLabel }) {
  try {
    return {
      name,
      ...parser(await readText(path)),
    };
  } catch {
    return {
      name,
      funcsPct: null,
      linesPct: null,
      metricLabel: fallbackMetricLabel,
    };
  }
}

function parseArgs(argv) {
  const options = {};

  for (let index = 0; index < argv.length; index += 1) {
    const token = argv[index];
    if (!token.startsWith("--")) {
      throw new Error(`unexpected argument: ${token}`);
    }

    const key = token.slice(2);
    const value = argv[index + 1];
    if (!value || value.startsWith("--")) {
      throw new Error(`missing value for --${key}`);
    }
    options[key] = value;
    index += 1;
  }

  return options;
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  const suites = [];

  if (args["go-summary"]) {
    suites.push(
      await loadSuite({
        name: "Go",
        path: args["go-summary"],
        parser: parseGoCoverageSummary,
        fallbackMetricLabel: "Statements unavailable",
      }),
    );
  }

  if (args["web-log"]) {
    suites.push(
      await loadSuite({
        name: "Web",
        path: args["web-log"],
        parser: parseBunCoverageSummary,
        fallbackMetricLabel: "Lines unavailable",
      }),
    );
  }

  if (args["contracts-log"]) {
    suites.push(
      await loadSuite({
        name: "Contracts",
        path: args["contracts-log"],
        parser: parseBunCoverageSummary,
        fallbackMetricLabel: "Lines unavailable",
      }),
    );
  }

  const output = {
    generatedAt: new Date().toISOString(),
    suites,
  };
  const markdown = renderCoverageMarkdown(output);

  if (args["output-json"]) {
    await mkdir(dirname(args["output-json"]), { recursive: true });
    await writeFile(args["output-json"], `${JSON.stringify(output, null, 2)}\n`, "utf8");
  }

  if (args["output-md"]) {
    await mkdir(dirname(args["output-md"]), { recursive: true });
    await writeFile(args["output-md"], markdown, "utf8");
  }

  process.stdout.write(markdown);
}

if (import.meta.main) {
  await main();
}
