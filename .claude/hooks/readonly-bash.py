#!/usr/bin/env python3
"""PreToolUse hook for Bash: allow only rerunning conformance tests and filtering their output.

The adjudicator needs a shell for one thing: rerunning a case, as in

    cd conformance && go test -count=1 -run 'TestSpecCases/strings/r4' -v . 2>&1 | grep -A3 REQ-STRINGS-004

It reads code with Read and Grep. So rather than a list of what a shell may not do, which review after review
found another way around, this is a list of what it may: these commands, with these options, and nothing else.

    cd conformance | cd difftest
    go test   with -count=N, -run PATTERN, -v, -timeout D, and the package . or ./...
    grep      with -A/-B/-C N, -E, -F, -i, -n, -v, -c, -w, -e PATTERN, a pattern, and files
    head/tail with -n N
    wc        with -l

Segments are joined by &&, | or ;. Environment assignments allowed: TZ, SPEC_SOURCES_REQUIRED. Redirections:
only 2>&1 and into /dev/null. No substitutions of any kind. Fails closed.
"""

from __future__ import annotations

import json
import re
import shlex
import sys

ALLOWED_ENV = {"SPEC_SOURCES_REQUIRED", "TZ"}
COUNT = re.compile(r"^-count=\d+$")
TIMEOUT = re.compile(r"^-timeout=\d+[smh]$")
NUMBER = re.compile(r"^\d+$")


def block(message: str) -> None:
    print(message, file=sys.stderr)
    sys.exit(2)


def check_go(args: list[str]) -> str | None:
    if not args or args[0] != "test":
        return "only go test is allowed"
    i, packages = 1, 0
    while i < len(args):
        a = args[i]
        if a == "-v" or COUNT.match(a) or TIMEOUT.match(a) or a.startswith("-run="):
            i += 1
        elif a in ("-run", "-timeout") and i + 1 < len(args):
            if a == "-timeout" and not TIMEOUT.match("-timeout=" + args[i + 1]):
                return f"-timeout {args[i + 1]} is not a duration"
            i += 2
        elif a in (".", "./..."):
            packages += 1
            i += 1
        else:
            return f"go test {a} is not allowed; only -count, -run, -v, -timeout and . or ./..."
    return None if packages <= 1 else "one package argument only"


def check_grep(args: list[str]) -> str | None:
    i = 0
    while i < len(args):
        a = args[i]
        if a in ("-A", "-B", "-C", "-e"):
            if i + 1 >= len(args) or (a != "-e" and not NUMBER.match(args[i + 1])):
                return f"grep {a} needs a value"
            i += 2
        elif re.fullmatch(r"-[EFinvcw]+", a) or re.fullmatch(r"-[ABC]\d+", a):
            i += 1
        elif a.startswith("-"):
            return f"grep {a} is not allowed"
        else:
            i += 1  # the pattern, then files: grep reads them and writes nothing
    return None


def check_segment(words: list[str]) -> str | None:
    while words and "=" in words[0] and not words[0].startswith("-"):
        name = words[0].split("=", 1)[0]
        if name not in ALLOWED_ENV:
            return f"setting {name} is not allowed"
        words = words[1:]
    if not words:
        return None
    cmd, args = words[0], words[1:]
    if cmd == "cd":
        return None if args in (["conformance"], ["difftest"]) else "only cd conformance or cd difftest"
    if cmd == "go":
        return check_go(args)
    if cmd == "grep":
        return check_grep(args)
    if cmd in ("head", "tail"):
        if not args or (len(args) == 2 and args[0] == "-n" and NUMBER.match(args[1])) or \
                (len(args) == 1 and re.fullmatch(r"-n?\d+", args[0])):
            return None
        return f"{cmd} takes only -n N"
    if cmd == "wc":
        return None if args in ([], ["-l"]) else "wc takes only -l"
    return f"`{cmd}` is not allowed; this shell reruns tests and filters their output, nothing else"


def segments(command: str) -> list[list[str]]:
    """Split a command into the words of each simple command, honouring quotes.

    Operators come out as their own tokens, so `grep -E 'a|b'` keeps its pattern whole. The only redirections
    kept are 2>&1 and into /dev/null, which write nothing; any other operator is refused.
    """
    lexer = shlex.shlex(command, posix=True, punctuation_chars=True)
    lexer.whitespace_split = True
    # shlex reads # as a comment anywhere in a word, bash only at a word's start: `grep x a#; touch y` would
    # hide the touch from this check and still run it. main() refuses # outright; this keeps the two agreeing.
    lexer.commenters = ""
    tokens = list(lexer)
    out: list[list[str]] = [[]]
    i = 0
    while i < len(tokens):
        tok = tokens[i]
        if tok in ("&&", "||", "|", ";"):
            out.append([])
            i += 1
        elif tok == ">&" and i + 1 < len(tokens) and tokens[i + 1] == "1" and out[-1][-1:] == ["2"]:
            out[-1].pop()  # 2>&1
            i += 2
        elif tok in (">", ">>") and i + 1 < len(tokens) and tokens[i + 1] == "/dev/null":
            if out[-1][-1:] in (["1"], ["2"]):
                out[-1].pop()
            i += 2
        elif tok and all(c in "();<>|&" for c in tok):
            raise PermissionError(f"`{tok}` is not allowed: no redirections, subshells or background commands")
        else:
            out[-1].append(tok)
            i += 1
    return [s for s in out if s]


def main() -> None:
    event = json.load(sys.stdin)
    command = event.get("tool_input", {}).get("command", "")
    agent = event.get("agent_type", "this agent")
    if any(s in command for s in ("`", "$", "\\", "\n", "#")):
        block(f"{agent}: substitutions, variables, escapes, comments and multi-line commands are not allowed")
    try:
        parts = segments(command)
    except PermissionError as exc:
        block(f"{agent}: {exc}")
    for words in parts:
        problem = check_segment(words)
        if problem:
            block(f"{agent}: {problem}. Read and Grep cover reading; report anything else you need.")


if __name__ == "__main__":
    try:
        main()
    except SystemExit:
        raise
    except Exception as exc:  # fail closed
        block(f"readonly-bash hook failed ({exc!r}); refusing the command")
