#!/usr/bin/env python3
"""PreToolUse hook: allow Read/Grep/Glob only under paths matching the given glob patterns.

Usage (called by role-policy.py):
    python3 only-read.py 'conformance/testdata/spec-cases/*' '!*known-failure*'

An allowlist, unlike deny-read.py: a Grep over "." or a Glob with no path would reach every file a
denylist forgot to name, so the agents that must not see the engine, its positions or another engine's
answers get only what is listed. A pattern starting with "!" carves an exclusion out of the allowed paths.

Patterns are matched case-insensitively against the path relative to the project root, with fnmatch, so
"*" also crosses "/". A Grep or Glob without a path searches the project root, which is never allowed.
A Glob pattern that is absolute or climbs with ".." is refused. A Grep reads every file under its path, so
one over a directory holding an excluded file is refused too: the agent greps a narrower path instead.
Fails closed.
"""

from __future__ import annotations

import json
import os
import sys
from fnmatch import fnmatchcase
from pathlib import Path


def block(message: str) -> None:
    print(message, file=sys.stderr)
    sys.exit(2)


def allowed(relative: str, patterns: list[str]) -> bool:
    include = [p for p in patterns if not p.startswith("!")]
    exclude = [p[1:] for p in patterns if p.startswith("!")]
    rel = relative.casefold()
    return any(fnmatchcase(rel, p.casefold()) for p in include) and not any(
        fnmatchcase(rel, p.casefold()) for p in exclude
    )


def main() -> None:
    event = json.load(sys.stdin)
    tool = event.get("tool_name", "")
    tool_input = event.get("tool_input", {})
    agent = event.get("agent_type", "this agent")
    patterns = sys.argv[1:]
    root = Path(os.environ.get("CLAUDE_PROJECT_DIR", event.get("cwd", "."))).resolve()

    if tool == "Read":
        raw = tool_input.get("file_path", "")
    else:
        raw = tool_input.get("path") or "."
        if tool == "Glob":
            pattern = str(tool_input.get("pattern", ""))
            if pattern.startswith("/") or pattern.startswith("~") or ".." in pattern:
                block(f"{agent}: a Glob pattern must be relative to an allowed path, without '..'")

    target = Path(raw).expanduser()
    target = (target if target.is_absolute() else root / target).resolve()
    try:
        relative = target.relative_to(root).as_posix()
    except ValueError:
        block(f"{agent} may not read outside the project: {target}")

    if not allowed(relative, patterns):
        listed = ", ".join(p for p in patterns if not p.startswith("!"))
        block(f"{agent} may not read {relative or 'the project root'}: it would bias an independent "
              f"verification. Readable: {listed}. Name a path inside one of them.")

    if tool == "Grep" and target.is_dir():
        for path in target.rglob("*"):
            inner = path.relative_to(root).as_posix()
            if path.is_file() and not allowed(inner, patterns):
                block(f"{agent} may not grep {relative}: it holds {inner}, which this agent may not read. "
                      "Grep a narrower path or a single file.")


if __name__ == "__main__":
    try:
        main()
    except SystemExit:
        raise
    except Exception as exc:  # fail closed
        block(f"only-read hook failed ({exc!r}); refusing")
