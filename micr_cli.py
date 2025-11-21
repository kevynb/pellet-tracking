#!/usr/bin/env python3
"""Minimal MICR CLI wrapper around the ultimateMICR SDK.

This script configures the SDK search paths, initializes the engine,
processes a single image, and prints the raw JSON result to stdout.
"""
from __future__ import annotations

import argparse
import ctypes
import json
import os
import sys
from pathlib import Path
from typing import Iterable, Tuple

import numpy as np
from PIL import Image, ImageOps

REPO_ROOT = Path(__file__).resolve().parent
SHIM_DIR = REPO_ROOT / "micr_cli_shim"
DEFAULT_SDK_ROOT = REPO_ROOT / "ultimateMICR-SDK"


def configure_sdk_paths(sdk_root: Path) -> Tuple[Path, Path]:
    """Add SDK and shim paths to ``sys.path`` and update ``LD_LIBRARY_PATH``.

    Returns the resolved binary and python directories for convenience.
    """
    bin_dir = sdk_root / "binaries" / "linux" / "x86_64"
    py_dir = sdk_root / "python"

    for path in (SHIM_DIR, bin_dir, py_dir):
        path_str = str(path)
        if path_str not in sys.path:
            sys.path.insert(0, path_str)

    ld_parts = [str(bin_dir)]
    current_ld = os.environ.get("LD_LIBRARY_PATH")
    if current_ld:
        ld_parts.append(current_ld)
    os.environ["LD_LIBRARY_PATH"] = ":".join(ld_parts)

    return bin_dir, py_dir


def preload_native_libraries(bin_dir: Path) -> None:
    """Explicitly load shared libraries from the SDK folder."""
    for lib_name in ("libtensorflow.so.1", "libtensorflow_framework.so.1", "libultimate_micr-sdk.so"):
        candidate = bin_dir / lib_name
        if candidate.exists():
            try:
                ctypes.CDLL(str(candidate))
            except OSError as exc:  # pragma: no cover - env dependent
                raise RuntimeError(f"Failed to load {lib_name}: {exc}") from exc


def ensure_paths_exist(paths: Iterable[Path]) -> None:
    missing = [str(p) for p in paths if not p.exists()]
    if missing:
        raise FileNotFoundError(f"Required paths are missing: {', '.join(missing)}")


def load_image_to_bgra(image_path: Path) -> Tuple[bytes, int, int]:
    """Load an image file and return a BGRA byte buffer with dimensions."""
    with Image.open(image_path) as img:
        img = ImageOps.exif_transpose(img).convert("RGBA")
        arr = np.array(img, dtype=np.uint8)
        bgra = arr[..., [2, 1, 0, 3]]  # RGBA -> BGRA
        height, width, _ = bgra.shape
        return bgra.tobytes(), width, height


def build_config(assets_folder: Path, fmt: str, gpgpu_enabled: bool, min_score: float, num_threads: int, backprop: bool, ielcd: bool, debug_level: str) -> str:
    cfg = {
        "debug_level": debug_level,
        "debug_write_input_image_enabled": False,
        "debug_internal_data_path": ".",
        "num_threads": num_threads,
        "gpgpu_enabled": gpgpu_enabled,
        "gpgpu_workload_balancing_enabled": False,
        "assets_folder": str(assets_folder),
        "format": fmt,
        "segmenter_accuracy": "high",
        "interpolation": "bilinear",
        "roi": [0, 0, 0, 0],
        "score_type": "min",
        "min_score": min_score,
        "backprop": backprop,
        "ielcd": ielcd,
    }
    return json.dumps(cfg)


def main() -> int:
    parser = argparse.ArgumentParser(description="Run MICR recognition with the ultimateMICR SDK")
    parser.add_argument("--image", required=True, help="Path to the input JPEG/PNG/BMP file")
    parser.add_argument("--assets", required=True, help="Path to the SDK assets directory")
    parser.add_argument("--format", default="cmc7", choices=["cmc7", "e13b", "e13b+cmc7"], help="MICR format to enable")
    parser.add_argument("--sdk-root", default=str(DEFAULT_SDK_ROOT), help="Path to the local ultimateMICR-SDK clone")
    parser.add_argument("--min-score", type=float, default=0.3, help="Minimum confidence threshold (0-1)")
    parser.add_argument("--gpgpu-enabled", action="store_true", help="Enable GPGPU acceleration if available")
    parser.add_argument("--threads", type=int, default=-1, help="Number of threads to allow (-1 lets the SDK decide)")
    parser.add_argument("--warmup", default=True, action=argparse.BooleanOptionalAction, help="Run warm-up on startup")
    parser.add_argument("--backprop", default=True, action=argparse.BooleanOptionalAction, help="Enable backpropagation (CMC-7 only)")
    parser.add_argument("--ielcd", default=True, action=argparse.BooleanOptionalAction, help="Enable low-contrast enhancement")
    parser.add_argument("--debug-level", default="info", choices=["info", "warn", "error", "fatal"], help="SDK debug verbosity")

    args = parser.parse_args()

    image_path = Path(args.image).expanduser().resolve()
    assets_path = Path(args.assets).expanduser().resolve()
    sdk_root = Path(args.sdk_root).expanduser().resolve()

    ensure_paths_exist([
        image_path,
        assets_path,
        sdk_root,
        sdk_root / "binaries" / "linux" / "x86_64",
        sdk_root / "python",
        SHIM_DIR,
    ])
    try:
        bin_dir, _ = configure_sdk_paths(sdk_root)
        preload_native_libraries(bin_dir)
    except Exception as exc:  # pragma: no cover - env guard
        print(json.dumps({"error": str(exc)}), file=sys.stderr)
        return 1

    from ultimateMicrSdk import UltMicrSdkEngine, ULTMICR_SDK_IMAGE_TYPE_BGRA32

    config_json = build_config(
        assets_folder=assets_path,
        fmt=args.format,
        gpgpu_enabled=args.gpgpu_enabled,
        min_score=args.min_score,
        num_threads=args.threads,
        backprop=args.backprop,
        ielcd=args.ielcd,
        debug_level=args.debug_level,
    )

    init_result = UltMicrSdkEngine.init(config_json)
    if not init_result.isOK():
        print(json.dumps({"error": init_result.phrase()}), file=sys.stderr)
        return 2

    try:
        if args.warmup and hasattr(UltMicrSdkEngine, "warmUp"):
            UltMicrSdkEngine.warmUp(ULTMICR_SDK_IMAGE_TYPE_BGRA32)

        buffer, width, height = load_image_to_bgra(image_path)
        result = UltMicrSdkEngine.process(
            ULTMICR_SDK_IMAGE_TYPE_BGRA32,
            buffer,
            width,
            height,
            0,
            1,
        )
        if not result.isOK():
            print(json.dumps({"error": result.phrase()}), file=sys.stderr)
            return 3

        print(result.json() or "{}")
        return 0
    finally:
        UltMicrSdkEngine.deInit()


if __name__ == "__main__":
    raise SystemExit(main())
