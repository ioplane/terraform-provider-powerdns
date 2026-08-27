"""Strict Semantic Versioning 2.0.0 syntax shared by publication tools."""

from __future__ import annotations

import re

NUMERIC = r"0|[1-9][0-9]*"
PRERELEASE_ID = rf"(?:{NUMERIC}|[0-9]*[A-Za-z-][0-9A-Za-z-]*)"
BUILD_ID = r"[0-9A-Za-z-]+"
SEMVER = re.compile(
    rf"\A(?:{NUMERIC})\.(?:{NUMERIC})\.(?:{NUMERIC})"
    rf"(?:-{PRERELEASE_ID}(?:\.{PRERELEASE_ID})*)?"
    rf"(?:\+{BUILD_ID}(?:\.{BUILD_ID})*)?\Z"
)
