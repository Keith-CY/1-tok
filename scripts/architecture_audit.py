#!/usr/bin/env python3
"""Utility used by .github/workflows/architecture-audit.yml.

This keeps the workflow functional even before a full production-grade audit
engine is added. It provides three subcommands:

- run: generate a snapshot and state files used by subsequent steps.
- dirty-gate: perform minimal dirty-tree checks for expected files.
- record-open-pr-block: record open-audit-PR block metadata.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import subprocess
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path
from typing import Any, Dict, List, Optional


def sha256_text(value: str) -> str:
    return hashlib.sha256(value.encode("utf-8")).hexdigest()


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as fp:
        for chunk in iter(lambda: fp.read(8192), b""):
            digest.update(chunk)
    return digest.hexdigest()


def read_json(path: Path) -> Dict[str, Any]:
    if not path.exists():
        return {}
    return json.loads(path.read_text(encoding="utf-8"))


def write_json(path: Path, payload: Dict[str, Any]) -> None:
    path.write_text(json.dumps(payload, indent=2, ensure_ascii=False), encoding="utf-8")


def git_paths_changed(*paths: str) -> List[str]:
    changed: List[str] = []
    for item in paths:
        # `git diff --name-only -- path` returns an empty string on clean.
        completed = subprocess.run(
            ["git", "diff", "--name-only", "--", item],
            check=False,
            capture_output=True,
            text=True,
        )
        if completed.stdout.strip():
            changed.append(item)
    return changed


@dataclass
class RunResult:
    changed: bool
    snapshot_path: Path
    state_path: Path


def cmd_run(args: argparse.Namespace) -> None:
    snapshot_path = Path(args.snapshot_file)
    state_path = Path(args.state_file)
    report_path = Path(args.report_file)
    body_path = Path(args.pr_body_file)

    snapshot_path.parent.mkdir(parents=True, exist_ok=True)
    state_path.parent.mkdir(parents=True, exist_ok=True)
    report_path.parent.mkdir(parents=True, exist_ok=True)
    body_path.parent.mkdir(parents=True, exist_ok=True)

    files = sorted(
        str(path.relative_to(Path(".")))
        for path in Path("docs").glob("*.md")
    )
    entries = []
    for path in files:
        digest = sha256_file(Path(path))
        entries.append(f"- `{path}`: {digest}")

    previous = read_json(state_path)
    previous_digest = previous.get("snapshot_digest", "")

    snapshot_body = [
        "# Architecture Audit Snapshot",
        "",
        f"Generated at: {datetime.utcnow().isoformat()}Z",
        "",
        "## Tracked docs",
        "",
        *entries if entries else ["- (no docs found)"],
        "",
    ]
    snapshot_path.write_text("\n".join(snapshot_body), encoding="utf-8")

    current_snapshot_digest = sha256_text(snapshot_path.read_text(encoding="utf-8"))
    changed = previous_digest != "" and current_snapshot_digest != previous_digest

    state_payload = {
        "generated_at": datetime.utcnow().isoformat() + "Z",
        "snapshot_digest": current_snapshot_digest,
        "snapshot_file": str(snapshot_path),
        "state_file": str(state_path),
        "tracked_files": files,
    }
    write_json(state_path, state_payload)

    report_payload = {
        "status": "ok",
        "generated_at": datetime.utcnow().isoformat() + "Z",
        "snapshot_changed": changed,
        "tracked_files": len(files),
        "compact_no_change": bool(args.compact_no_change),
    }
    write_json(report_path, report_payload)

    pr_body = [
        "## Architecture Audit",
        "",
        "- status: ok",
        f"- tracked files: {len(files)}",
        f"- snapshot changed: {changed}",
    ]
    body_path.write_text("\n".join(pr_body) + "\n", encoding="utf-8")

    print(json.dumps({"status": "ok", "changed": changed, "snapshot_digest": current_snapshot_digest}))


def cmd_dirty_gate(args: argparse.Namespace) -> None:
    state = read_json(Path(args.state_file))
    expected_paths = args.expected_path or []
    dirty_paths = git_paths_changed(*expected_paths)
    blocked = bool(dirty_paths)
    payload = {
        "blocked": blocked,
        "reason_code": "CLEAN_DIRTY_GATE" if not blocked else "REQUIRES_MANUAL_INSPECTION",
        "dirty_count": len(dirty_paths),
        "representative_paths": dirty_paths[:10],
        "state_snapshot_digest": state.get("snapshot_digest"),
    }
    if args.auto_stash_env:
        marker = f"{args.auto_stash_env}/architecture-audit-dirty-gate.txt"
        Path(args.auto_stash_env).mkdir(parents=True, exist_ok=True)
        Path(marker).write_text("\n".join(dirty_paths) + "\n", encoding="utf-8")
    write_json(Path(args.write_json), payload)
    print(json.dumps(payload))


def cmd_record_open_pr_block(args: argparse.Namespace) -> None:
    from datetime import timezone

    state = read_json(Path(args.state_file))
    now = datetime.now(tz=timezone.utc).isoformat()
    message = (
        f"Existing open audit PR is blocked for review: {args.pr_url} "
        f"(owner={args.pr_owner}, created_at={args.pr_created_at}). "
        f"Next action: {args.required_next_action}"
    )
    payload = {
        "state": state,
        "pr_url": args.pr_url,
        "pr_owner": args.pr_owner,
        "pr_created_at": args.pr_created_at,
        "warning_hours": args.warning_hours,
        "critical_hours": args.critical_hours,
        "required_next_action": args.required_next_action,
        "message": message,
        "recorded_at": now,
    }
    write_json(Path(args.write_json), payload)
    print(json.dumps(payload))


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="command", required=True)

    run = sub.add_parser("run")
    run.add_argument("mode", choices=["run"], nargs="?")
    run.add_argument("--state-file", required=True)
    run.add_argument("--snapshot-file", required=True)
    run.add_argument("--report-file", required=True)
    run.add_argument("--pr-body-file", required=True)
    run.add_argument("--compact-no-change", action="store_true")
    run.set_defaults(func=cmd_run)

    dirty = sub.add_parser("dirty-gate")
    dirty.add_argument("--state-file", required=True)
    dirty.add_argument("--stage", required=True)
    dirty.add_argument("--expected-path", action="append", default=[])
    dirty.add_argument("--auto-stash-env", default="")
    dirty.add_argument("--write-json", required=True)
    dirty.set_defaults(func=cmd_dirty_gate)

    block = sub.add_parser("record-open-pr-block")
    block.add_argument("--state-file", required=True)
    block.add_argument("--pr-url", required=True)
    block.add_argument("--pr-owner", required=True)
    block.add_argument("--pr-created-at", required=True)
    block.add_argument("--warning-hours", required=True)
    block.add_argument("--critical-hours", required=True)
    block.add_argument("--required-next-action", required=True)
    block.add_argument("--write-json", required=True)
    block.set_defaults(func=cmd_record_open_pr_block)

    return parser


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()
    args.func(args)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
