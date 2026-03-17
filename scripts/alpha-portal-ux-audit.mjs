import { readdirSync, readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';

const root = path.resolve('apps/web/app');
const includeRoots = ['buyer', 'provider', 'ops'];

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

function main() {
  const targetFiles = includeRoots.flatMap((r) => walk(path.join(root, r)));

  const strictMode = process.env.ALPHA_UX_AUDIT_STRICT === '1';

  const report = {
    strictMode,
    timestamp: new Date().toISOString(),
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

        const canonicalLabels = new Set([
          'Clear filters',
          'Clear bid filters',
          'Clear review filters',
          'Clear risk filters',
          'Clear dispute filters',
          'Clear funding filters',
        ]);
        const canonicalHrefPatterns = [
          /^\/buyer(?:(?:#|\?|$).*)?/,
          /^\/provider(?:(?:#|\?|$).*)?/,
          /^\/ops(?:(?:#|\?|$).*)?/,
          /^\/login(?:(?:#|\?|$).*)?/,
          /^\/$/,
        ];

        if (actionLabel && !canonicalLabels.has(actionLabel) && !actionLabel.startsWith('Create') && !actionLabel.startsWith('Track') && !actionLabel.startsWith('Open')) {
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
  console.error(err);
  process.exit(2);
}
