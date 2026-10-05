#!/usr/bin/env python3
"""Download the pinned specification texts that spec cases cite.

    python3 scripts/spec-sources.py

Reads conformance/testdata/spec-cases/SPEC_SOURCES and writes, for each source,
a plain-text rendering (<name>.txt) into conformance/testdata/spec-cases/.spec/.
A source whose bytes do not match its pinned sha256 is refused, and any text
left from an earlier download is removed, so quotes are never checked against
a text the pin no longer names.

The .txt is what authors quote from and what TestSpecCaseCatalog checks quotes
against, so it is produced here and only here. HTML loses its tags and entities;
Markdown and AsciiDoc are kept as written. Either way the comparison collapses
whitespace, so a quote may span lines.
"""

from __future__ import annotations

import hashlib
import html
import re
import sys
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
CASES = ROOT / "conformance" / "testdata" / "spec-cases"


def to_text(raw: str) -> str:
    raw = re.sub(r"(?is)<(script|style)\b.*?</\1>", " ", raw)
    raw = re.sub(r"(?i)<br\s*/?>|</(p|div|li|h[1-6]|tr|td|th|pre|table)>", "\n", raw)
    text = html.unescape(re.sub(r"<[^>]+>", " ", raw))
    lines = (re.sub(r"[ \t ]+", " ", line).strip() for line in text.splitlines())
    return "\n".join(line for line in lines if line)


def sources() -> list[tuple[str, str, str]]:
    out = []
    for line in (CASES / "SPEC_SOURCES").read_text().splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        name, digest, url = line.split()
        out.append((name, digest, url))
    return out


def main() -> int:
    dest = CASES / ".spec"
    dest.mkdir(exist_ok=True)
    failed = 0
    for name, digest, url in sources():
        out = dest / f"{name}.txt"
        out.unlink(missing_ok=True)
        try:
            with urllib.request.urlopen(url, timeout=60) as resp:  # noqa: S310 - pinned hosts
                raw = resp.read()
        except Exception as exc:  # report and keep going: one missing source must not hide the others
            print(f"  {name:16} FAILED: {exc}")
            failed += 1
            continue
        got = hashlib.sha256(raw).hexdigest()
        if got != digest:
            print(f"  {name:16} REFUSED: sha256 {got} is not the pinned {digest}")
            failed += 1
            continue
        text = raw.decode("utf-8")
        if url.endswith(".html"):
            text = to_text(text)
        out.write_text(text, encoding="utf-8")
        print(f"  {name:16} {len(raw):>8} bytes")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
