#!/usr/bin/env python3
"""PreToolUse hook (project settings): apply each /fhirpath-cases agent's tool policy, keyed on `agent_type`.

Ported from gofhir/server's conformance workflow. Policies live here rather than in each agent's frontmatter
because frontmatter hooks did not fire for background subagents; session hooks do, and receive `agent_type`.
Calls from the main agent (no `agent_type`) and from agents outside the workflow pass through.

The point of the policy is independence. A spec case states what the specification says; an author who has
seen what this engine answers, what fhirpath.js answers, or which positions CONFORMANCE.md takes writes
cases that agree with one of them instead. So the author and the verifier read only the specification texts,
the suite's own cases and inputs, the format, and their task's files. Fails closed.
"""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path

HOOKS = Path(__file__).resolve().parent
WRITE_TOOLS = {"Edit", "Write", "NotebookEdit"}
READ_TOOLS = {"Read", "Grep", "Glob"}

# What the blind agents may read. Never the engine, its documentation, CONFORMANCE.md, difftest (which holds
# fhirpath.js), the baselines or reasons, the run outputs, or the adjudication.
BLIND_READ = [
    "conformance/testdata/spec-cases",
    "conformance/testdata/spec-cases/*",
    "conformance/testdata/fhirpath-suite",
    "conformance/testdata/fhirpath-suite/*",
    "conformance/testdata/fhirpath-suite-r5",
    "conformance/testdata/fhirpath-suite-r5/*",
    "grammar/fhirpath.g4",
    ".workflow/*",
    "!*known-failure*",
    "!.workflow/*/context*",
    "!.workflow/*/03-adjudication*",
]

# agent_type -> (paths it may write, paths it may read or None for anything, Bash allowed?)
POLICIES = {
    "case-author": (
        ["conformance/testdata/spec-cases/*.xml", ".workflow/*/01-cases.md"],
        BLIND_READ,
        False,
    ),
    # The verifier judges the cases, not the author's account of them.
    "case-verifier": (
        [".workflow/*/02-check.md"],
        BLIND_READ + ["!.workflow/*/01-cases*"],
        False,
    ),
    # The adjudicator reads everything, the engine and fhirpath.js included, and alone writes the reasons.
    "case-adjudicator": (
        ["conformance/testdata/spec-cases/*.known-failure-reasons.txt",
         ".workflow/*/03-adjudication.md", ".workflow/*/03-case-defects.md"],
        None,
        True,
    ),
}

# 03-case-defects.md goes back to the author, who must stay blind: it states each defect in terms of the
# specification and the case, never the engine or what either engine answered.
LEAKS = ("eval/", "funcs/", "types/", "parser/", "internal/", ".go", "fhirpath.js", "difftest", "node_modules",
         "CONFORMANCE", "this engine", "the engine")


def run(script: str, args: list[str], raw: str) -> None:
    result = subprocess.run([sys.executable, str(HOOKS / script), *args], input=raw, capture_output=True, text=True)
    if result.returncode != 0:
        print(result.stderr.strip() or f"{script} refused the call", file=sys.stderr)
        sys.exit(2)


def main() -> None:
    raw = sys.stdin.read()
    event = json.loads(raw)
    agent = event.get("agent_type")
    tool = event.get("tool_name", "")
    if not agent or agent not in POLICIES:
        return

    writable, readable, bash = POLICIES[agent]
    tool_input = event.get("tool_input", {})

    if tool in WRITE_TOOLS:
        run("only-paths.py", writable, raw)
        path = str(tool_input.get("file_path", ""))
        # Case-insensitive, as only-paths.py is: on macOS 03-Case-Defects.md is the same file.
        if path.casefold().endswith("03-case-defects.md"):
            text = " ".join(str(tool_input.get(k, "")) for k in ("content", "new_string"))
            leaked = [w for w in LEAKS if w.casefold() in text.casefold()]
            if leaked:
                print(f"{agent}: 03-case-defects.md is read by the case author, who must not learn what either "
                      f"engine does; remove {', '.join(leaked)} and state each defect in terms of the "
                      "specification and the case.", file=sys.stderr)
                sys.exit(2)
    if tool == "Bash":
        if not bash:
            print(f"{agent} has no shell in this workflow.", file=sys.stderr)
            sys.exit(2)
        run("readonly-bash.py", [], raw)
    if tool in READ_TOOLS and readable is not None:
        run("only-read.py", readable, raw)


if __name__ == "__main__":
    try:
        main()
    except SystemExit:
        raise
    except Exception as exc:  # fail closed
        print(f"role-policy hook failed ({exc!r}); refusing", file=sys.stderr)
        sys.exit(2)
