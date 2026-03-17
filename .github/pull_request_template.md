## Summary

- [ ] Briefly summarize what changed and why.

## Scope

- [ ] Filesystem scope is limited to this PR.

## Verification

- [ ] Relevant local checks run
  - [ ] `bun run portal:check`
  - [ ] `bun run portal:check:strict`
  - [ ] `bun run portal:check:quick`
  - [ ] `bun run portal:check:fast`

## PR Readiness Checklist

- [ ] PR description includes key commit changes
- [ ] Issue/Task linkage is provided (if applicable)
- [ ] Required discussions or follow-ups are documented
- [ ] If this PR modifies any `alpha-portal-ux-audit*.{config,json,schema,example}` or `scripts/alpha-portal-ux-audit.mjs`, include:
  - [ ] the exact list of allowed label/path changes
  - [ ] why it is backward-compatible
  - [ ] impact on strict-mode CI checks



## Portal UX Governance Evidence Guide (if applicable)

When touching `buyer/provider/ops` pages or `alpha-portal-ux-audit*` governance files, pick the relevant row and fill verification artifacts inline:

| CI Mode | What changed | Required checks |
| --- | --- | --- |
| `ui` | Portal page scope changed (`apps/web/app/{buyer,provider,ops}/...`) | `bun run portal:check:strict` (or `bun run portal:check`)
| `config` | Audit config/schema/script/CI touched | `bun run alpha:ux-audit:validate-config` + `bun run alpha:ux-audit:strict`
| `full` | Both scopes changed | `bun run portal:check:strict` + `bun run alpha:ux-audit:validate-config`
| `none` | None of above scopes changed | N/A (CI governance mode should be `none`)

Add links to artifacts if CI comments contain failed/passing `Portal UX governance mode`.

## Alpha Portal UX Audit Rule Change Impact Template

<!-- Fill this section only if this PR touches UX audit rules/config/script. -->

- Canonical labels changed:
  - Added:
  - Removed:
  - Renamed:
- Canonical href patterns changed:
  - Added:
  - Removed:
  - Modified:
- Backward compatibility rationale:
- Strict-mode impact:
  - Expected new failures in existing empty-state actions:
  - Whether `portal:check:strict` remains green without app changes:
- Validation evidence:
  - `bun run portal:check:strict`
  - `bun run portal:check:quick`

## Notes

- If this PR is in response to an issue, add label `docs`, `bug`, `enhancement`, etc.
- For high-priority changes include any relevant risk notes.
Fixes #
