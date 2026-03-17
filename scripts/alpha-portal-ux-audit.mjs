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
    } else if (e.isFile() && e.name === 'page.tsx') {
      files.push(p);
    }
  }
  return files;
}

function main() {
  const targetFiles = includeRoots.flatMap((r) => walk(path.join(root, r)));

  const report = {
    timestamp: new Date().toISOString(),
    scope: ['buyer/*', 'provider/*', 'ops/*'],
    summary: { filesChecked: 0, missingAriaCurrent: 0, totalEmptyStates: 0, hashOnlyEmptyStateLinks: 0 },
    issues: [],
    emptyStates: [],
  };

  for (const file of targetFiles) {
    const text = readFileSync(file, 'utf8');
    const lines = text.split('\n');
    report.summary.filesChecked += 1;

    for (let i = 0; i < lines.length; i++) {
      const line = lines[i];
      if (line.includes('<a ') && line.includes('className={chipClass(') && !line.includes('aria-current')) {
        report.summary.missingAriaCurrent += 1;
        report.issues.push({
          file: path.relative('.', file),
          line: i + 1,
          type: 'chip-missing-aria-current',
          snippet: line.trim(),
        });
      }

      if (line.includes('<EmptyState')) {
        let j = i;
        let message;
        let actionLabel;
        let actionHref;
        while (j < Math.min(i + 8, lines.length)) {
          const t = lines[j];
          const m1 = t.match(/message=\"([^\"]*)\"/);
          const m2 = t.match(/actionLabel=\"([^\"]*)\"/);
          const m3 = t.match(/actionHref=\"([^\"]*)\"/);
          if (m1) message = m1[1];
          if (m2) actionLabel = m2[1];
          if (m3) actionHref = m3[1];
          if (t.includes('/>') || t.includes('</EmptyState>')) break;
          j += 1;
        }
        report.summary.totalEmptyStates += 1;
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
        report.emptyStates.push({ file: path.relative('.', file), line: i + 1, message, actionLabel, actionHref });
      }
    }
  }

  const out = JSON.stringify(report, null, 2);
  writeFileSync('alpha-portal-ux-audit.json', out);
  console.log(out);

  if (report.summary.missingAriaCurrent || report.summary.hashOnlyEmptyStateLinks) {
    process.exit(1);
  }
}

try {
  main();
} catch (err) {
  console.error(err);
  process.exit(2);
}
