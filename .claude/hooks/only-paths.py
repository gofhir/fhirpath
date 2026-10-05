#!/usr/bin/env python3
"""PreToolUse hook: allow Edit/Write only on paths matching the given glob patterns.

Usage (called by role-policy.py, or from a hook definition):
    python3 "$CLAUDE_PROJECT_DIR/.claude/hooks/only-paths.py" 'tests/**' '.workflow/**'
    python3 "$CLAUDE_PROJECT_DIR/.claude/hooks/only-paths.py" --deny 'tests/acceptance/**'

Without --deny the patterns are an allowlist; with --deny they are a denylist.

Patterns are matched case-insensitively against the path relative to the project root. Exit code 2 blocks the tool call
and the message on stderr is shown to the agent. Any unexpected error also blocks (fail closed):
exit code 1 would be treated as a non-blocking error and let the write through.
"""

import json
import os
import sys
from fnmatch import fnmatchcase
from pathlib import Path


def block(message: str) -> None:
    print(message, file=sys.stderr)
    sys.exit(2)


def main() -> None:
    event = json.load(sys.stdin)
    tool_input = event.get("tool_input", {})
    raw_path = tool_input.get("file_path") or tool_input.get("notebook_path")
    if not raw_path:
        block(f"only-paths hook: no file path in {event.get('tool_name')} input; refusing")

    root = Path(os.environ.get("CLAUDE_PROJECT_DIR", event.get("cwd", "."))).resolve()
    target = Path(raw_path)
    target = (target if target.is_absolute() else root / target).resolve()
    agent = event.get("agent_type", "this agent")
    deny = sys.argv[1:2] == ["--deny"]
    patterns = sys.argv[2:] if deny else sys.argv[1:]

    try:
        relative = target.relative_to(root).as_posix()
    except ValueError:
        block(f"{agent} may not write outside the project: {target}")

    # Case-insensitive: on macOS TESTS/acceptance and tests/acceptance are the same directory.
    include = [p for p in patterns if not p.startswith("!")]
    exclude = [p[1:] for p in patterns if p.startswith("!")]
    matched = any(fnmatchcase(relative.casefold(), p.casefold()) for p in include)
    if not deny:
        # Allow mode: "!pattern" carves an exclusion out of the allowed paths.
        matched = matched and not any(fnmatchcase(relative.casefold(), p.casefold()) for p in exclude)
    if matched != deny:
        sys.exit(0)

    rule = f"Forbidden paths: {', '.join(patterns)}" if deny else f"Allowed paths: {', '.join(patterns)}"
    block(
        f"{agent} may not write {relative}. {rule}. "
        "If a change there is needed, report it instead of making it."
    )


if __name__ == "__main__":
    try:
        main()
    except SystemExit:
        raise
    except Exception as exc:  # fail closed
        block(f"only-paths hook failed ({exc!r}); refusing the write")
