import { readdirSync, readFileSync, writeFileSync, existsSync } from 'node:fs';
import { execSync } from 'node:child_process';
import path from 'node:path';

const root = path.resolve('apps/web/app');
const includeRoots = ['buyer', 'provider', 'ops'];

const DEFAULT_CONFIG_PATH = './alpha-portal-ux-audit.config.json';

const DEFAULT_CANONICAL_LABELS = [
  'Clear filters',
  'Clear bid filters',
  'Clear review filters',
  'Clear risk filters',
  'Clear dispute filters',
  'Clear funding filters',
  'Track opportunities',
  'Create RFQ now',
  'Create an RFQ',
  'Open treasury controls',
];

const DEFAULT_CANONICAL_HREF_PATTERNS = [
  /^\/buyer(?:(?:#|\?|$).*)?/,
  /^\/provider(?:(?:#|\?|$).*)?/,
  /^\/ops(?:(?:#|\?|$).*)?/,
  /^\/login(?:(?:#|\?|$).*)?/,
  /^\/$/,
];

function loadConfig() {
  const candidatePath = process.env.ALPHA_UX_AUDIT_CONFIG || process.env.ALPHA_UX_AUDIT_CONFIG_PATH || DEFAULT_CONFIG_PATH;
  if (!existsSync(candidatePath)) {
    return {
      canonicalLabels: DEFAULT_CANONICAL_LABELS,
      canonicalHrefPatterns: DEFAULT_CANONICAL_HREF_PATTERNS,
      configPath: null,
      source: 'default',
    };
  }

  let parsed;

  try {
    const raw = readFileSync(candidatePath, 'utf8');
    parsed = JSON.parse(raw);
  } catch (error) {
    throw new Error(`Cannot read or parse config: ${candidatePath}. ${error?.message || error}`);
  }

  if (!Array.isArray(parsed.canonicalLabels) || !Array.isArray(parsed.canonicalHrefPatterns)) {
    throw new Error(`Invalid config schema at ${candidatePath}: canonicalLabels and canonicalHrefPatterns must be arrays.`);
  }

  if (parsed.canonicalLabels.length === 0) {
    throw new Error(`Invalid config schema at ${candidatePath}: canonicalLabels cannot be empty.`);
  }

  const invalidLabel = parsed.canonicalLabels.find((label) => typeof label !== 'string' || label.trim().length === 0);
  if (invalidLabel) {
    throw new Error(`Invalid config schema at ${candidatePath}: canonicalLabels contains invalid value ${JSON.stringify(invalidLabel)}.`);
  }

  const invalidPattern = parsed.canonicalHrefPatterns.find((pattern) => typeof pattern !== 'string' || pattern.trim().length === 0);
  if (invalidPattern) {
    throw new Error(`Invalid config schema at ${candidatePath}: canonicalHrefPatterns contains invalid value ${JSON.stringify(invalidPattern)}.`);
  }

  let canonicalHrefPatterns;
  try {
    canonicalHrefPatterns = parsed.canonicalHrefPatterns.map((pattern) => new RegExp(pattern));
  } catch (error) {
    throw new Error(`Invalid config schema at ${candidatePath}: canonicalHrefPatterns has invalid regex. ${error.message}`);
  }

  return {
    canonicalLabels: parsed.canonicalLabels,
    canonicalHrefPatterns,
    configPath: candidatePath,
    source: candidatePath,
  };
}


function walk(dir) {
  const entries = readdirSync(dir, { withFileTypes: true });
  const files = [];

  for (const e of entries) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) {
      files.push(...walk(p));
      continue;
    }
    if (e.isFile() && e.name === 'page.tsx') {
      files.push(p);
    }
  }

  return files;
}

function formatSummaryText(report) {
  const status = (report.summary.missingAriaCurrent ||
    report.summary.missingEmptyStateActionLabel ||
    report.summary.missingEmptyStateActionHref ||
    report.summary.hashOnlyEmptyStateLinks ||
    (report.strictMode && (report.summary.nonCanonicalActionLabels || report.summary.nonCanonicalActionHrefs)))
    ? 'FAIL' : 'PASS';

  const lines = [];
  const branchInfo = report?.git?.gitBranch || 'unknown';
  const revInfo = report?.git?.gitRev || 'unknown';
  lines.push('# Alpha Portal UX Audit Summary');
  lines.push('');
  lines.push(`Status: **${status}**`);
  lines.push(`Timestamp: ${report.timestamp}`);
  lines.push(`Scope snapshot: ${branchInfo}@${revInfo}`);
  lines.push(`Rule config: ${report.config.source}${report.config.path ? `@${report.config.path}` : ""}`);
  lines.push('');
  lines.push('## Scope');
  lines.push(`- checkedFiles: ${report.summary.filesChecked}`);
  lines.push(`- totalEmptyStates: ${report.summary.totalEmptyStates}`);
  lines.push('');
  lines.push('## Metrics');
  lines.push(`- missingAriaCurrent: ${report.summary.missingAriaCurrent}`);
  lines.push(`- missingEmptyStateActionLabel: ${report.summary.missingEmptyStateActionLabel}`);
  lines.push(`- missingEmptyStateActionHref: ${report.summary.missingEmptyStateActionHref}`);
  lines.push(`- hashOnlyEmptyStateLinks: ${report.summary.hashOnlyEmptyStateLinks}`);
  lines.push(`- nonCanonicalActionLabels: ${report.summary.nonCanonicalActionLabels}`);
  lines.push(`- nonCanonicalActionHrefs: ${report.summary.nonCanonicalActionHrefs}`);
  lines.push(`- strictMode: ${report.strictMode}`);
  lines.push('');

  if (report.issues.length === 0) {
    lines.push('## Issues');
    lines.push('None');
    return lines.join('\n');
  }

  const groups = [
    ['chip-missing-aria-current', 'Missing aria-current'],
    ['empty-state-missing-action-label', 'Missing EmptyState actionLabel'],
    ['empty-state-missing-action-href', 'Missing EmptyState actionHref'],
    ['empty-state-hash-only-link', 'Hash-only EmptyState href'],
    ['non-canonical'],
  ];

  for (const [type, title] of groups) {
    const items = report.issues.filter((i) =>
      title === 'non-canonical'
        ? (i.type === 'empty-state-nonCanonicalActionLabel' || i.type === 'empty-state-nonCanonicalActionHref')
        : i.type === type
    );
    if (title === 'non-canonical') continue;
    if (items.length === 0) continue;
    lines.push(`## ${title}`);
    for (const i of items) {
      lines.push(`- ${i.file}:${i.line} :: ${i.type}`);
    }
    lines.push('');
  }

  if (report.nonCanonicalActionLabels.length) {
    lines.push('## Non-canonical action labels');
    for (const i of report.nonCanonicalActionLabels) {
      lines.push(`- ${i.file}:${i.line} :: ${i.actionLabel}`);
    }
    lines.push('');
  }
  if (report.nonCanonicalActionHrefs.length) {
    lines.push('## Non-canonical action hrefs');
    for (const i of report.nonCanonicalActionHrefs) {
      lines.push(`- ${i.file}:${i.line} :: ${i.actionHref}`);
    }
  }

  return lines.join('\n');
}


function getGitMeta() {
  try {
    const gitRev = execSync('git rev-parse --short HEAD', { encoding: 'utf8' }).trim();
    const gitBranch = execSync('git rev-parse --abbrev-ref HEAD', { encoding: 'utf8' }).trim();
    return { gitRev, gitBranch };
  } catch {
    return { gitRev: 'unknown', gitBranch: 'unknown' };
  }
}

function main() {
  const targetFiles = includeRoots.flatMap((r) => walk(path.join(root, r)));

  const strictMode = process.env.ALPHA_UX_AUDIT_STRICT === '1';
  const config = loadConfig();

  const report = {
    strictMode,
    timestamp: new Date().toISOString(),
    git: getGitMeta(),
    config: {
      source: config.source,
      path: config.configPath,
    },
    scope: ['buyer/*', 'provider/*', 'ops/*'],
    summary: {
      filesChecked: 0,
      missingAriaCurrent: 0,
      missingEmptyStateActionLabel: 0,
      missingEmptyStateActionHref: 0,
      totalEmptyStates: 0,
      hashOnlyEmptyStateLinks: 0,
      nonCanonicalActionLabels: 0,
      nonCanonicalActionHrefs: 0,
    },
    issues: [],
    emptyStates: [],
    nonCanonicalActionLabels: [],
    nonCanonicalActionHrefs: [],
  };

  for (const file of targetFiles) {
    const text = readFileSync(file, 'utf8');
    const lines = text.split('\n');
    report.summary.filesChecked += 1;

    for (let i = 0; i < lines.length; i++) {
      const startLine = lines[i];
      if (startLine.includes('<a ')) {
        let j = i;
        let anchorBlock = '';
        while (j < lines.length) {
          anchorBlock += lines[j] + '\n';
          if (lines[j].includes('/>') || lines[j].includes('>')) {
            break;
          }
          j += 1;
        }

        if (anchorBlock.includes('className={chipClass(') && !anchorBlock.includes('aria-current')) {
          report.summary.missingAriaCurrent += 1;
          report.issues.push({
            file: path.relative('.', file),
            line: i + 1,
            type: 'chip-missing-aria-current',
            snippet: anchorBlock.trim().split('\n')[0],
          });
        }

        i = j;
        continue;
      }

      if (startLine.includes('<EmptyState')) {
        let j = i;
        let message;
        let actionLabel;
        let actionHref;
        while (j < Math.min(i + 10, lines.length)) {
          const t = lines[j];
          const m1 = t.match(/message="([^"]*)"/);
          const m2 = t.match(/actionLabel="([^"]*)"/);
          const m3 = t.match(/actionHref="([^"]*)"/);

          if (m1) message = m1[1];
          if (m2) actionLabel = m2[1];
          if (m3) actionHref = m3[1];

          if (t.includes('/>') || t.includes('</EmptyState>')) {
            break;
          }
          j += 1;
        }

        report.summary.totalEmptyStates += 1;

        if (!actionLabel) {
          report.summary.missingEmptyStateActionLabel += 1;
          report.issues.push({
            file: path.relative('.', file),
            line: i + 1,
            type: 'empty-state-missing-action-label',
            message,
          });
        }

        if (!actionHref) {
          report.summary.missingEmptyStateActionHref += 1;
          report.issues.push({
            file: path.relative('.', file),
            line: i + 1,
            type: 'empty-state-missing-action-href',
            message,
            actionLabel,
          });
        }

        if ((actionHref || '').startsWith('#')) {
          report.summary.hashOnlyEmptyStateLinks += 1;
          report.issues.push({
            file: path.relative('.', file),
            line: i + 1,
            type: 'empty-state-hash-only-link',
            message,
            actionLabel,
            actionHref,
          });
        }

        const canonicalLabels = new Set(config.canonicalLabels);
        const canonicalHrefPatterns = config.canonicalHrefPatterns;

        if (actionLabel && !canonicalLabels.has(actionLabel)) {
          report.summary.nonCanonicalActionLabels += 1;
          report.nonCanonicalActionLabels.push({
            file: path.relative('.', file),
            line: i + 1,
            message,
            actionLabel,
            actionHref,
          });
        }

        if (actionHref && !canonicalHrefPatterns.some((re) => re.test(actionHref)) && !actionHref.startsWith('http')) {
          report.summary.nonCanonicalActionHrefs += 1;
          report.nonCanonicalActionHrefs.push({
            file: path.relative('.', file),
            line: i + 1,
            message,
            actionLabel,
            actionHref,
          });
        }

        report.emptyStates.push({ file: path.relative('.', file), line: i + 1, message, actionLabel, actionHref });

        i = j;
        continue;
      }
    }
  }


  const out = JSON.stringify(report, null, 2);
  writeFileSync('alpha-portal-ux-audit.json', out);

  const summaryText = formatSummaryText(report);
  writeFileSync('alpha-portal-ux-audit-summary.md', summaryText + '\n');
  console.log(out);

  if (
    report.summary.missingAriaCurrent ||
    report.summary.missingEmptyStateActionLabel ||
    report.summary.missingEmptyStateActionHref ||
    report.summary.hashOnlyEmptyStateLinks ||
    (strictMode && (
      report.summary.nonCanonicalActionLabels ||
      report.summary.nonCanonicalActionHrefs
    ))
  ) {
    process.exit(1);
  }
}

try {
  main();
} catch (err) {
  console.error('[alpha:ux-audit] fatal:', err?.message || err);
  process.exit(2);
}
