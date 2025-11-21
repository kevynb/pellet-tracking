"""Minimal shim to satisfy legacy ``import imp`` calls on Python 3.12+.

The upstream SWIG wrapper uses ``imp.find_module``/``load_module``.
We raise ``ImportError`` in ``find_module`` so the wrapper falls back
to a regular ``import``; ``load_module`` is provided for completeness
and defers to ``importlib.import_module``.
"""
from __future__ import annotations
import importlib
from types import ModuleType
from typing import Iterable, Optional, Tuple

# Keep a small subset of the original constants for callers that expect them.
C_EXTENSION = 3
PY_SOURCE = 1
PY_COMPILED = 2
PKG_DIRECTORY = 5
C_BUILTIN = 6
PY_FROZEN = 7
IMP_HOOK = 9


def find_module(name: str, path: Optional[Iterable[str]] = None):  # pragma: no cover - compatibility shim
    raise ImportError("imp module removed; use importlib instead")


def load_module(name: str, file=None, pathname: Optional[str] = None, description: Optional[Tuple[str, str, int]] = None) -> ModuleType:  # pragma: no cover - compatibility shim
    return importlib.import_module(name)
