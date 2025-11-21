# MICR CLI Overview

This document describes the architecture and usage of the MICR command-line interface (`micr-cli`) that wraps the Doubango ultimateMICR SDK.

## Goals
- Provide a single-entry CLI to run MICR recognition on static images.
- Keep configuration minimal while exposing common tuning flags (format, GPU toggle, score threshold).
- Ensure results are emitted as JSON on stdout for easy piping or redirection.

## Prerequisites and build steps
- Run `./scripts/setup_micr.sh` from the repo root. It:
  - Clones or refreshes a shallow copy (`--depth 1`) of the upstream `ultimateMICR-SDK` repository locally without relying on a git submodule.
  - Ensures `uv` is installed, creates `.venv/`, and installs Python dependencies (NumPy, Pillow, Cython, setuptools) via `uv pip`.
  - Downloads the TensorFlow CPU runtime (`libtensorflow.so.1` and friends) automatically into `ultimateMICR-SDK/binaries/linux/x86_64/`.
  - Builds the `_ultimateMicrSdk.so` Python extension in-place using the virtual environment's Python.
- The shim directory (`micr_cli_shim`) remains on `PYTHONPATH` to satisfy the SDK's legacy `import imp` call under Python 3.12.

## Runtime flow
1. `micr_cli.py` prepends the shim and SDK paths to `sys.path` and updates `LD_LIBRARY_PATH` so the native libraries load cleanly.
2. A JSON configuration string is built from CLI flags (format, GPU toggle, confidence threshold, thread count, backprop/IELCD toggles).
3. The MICR engine is initialized, optionally warm-ups using BGRA32, and then processes a single image loaded via Pillow/NumPy into a BGRA byte buffer.
4. Raw JSON from `UltMicrSdkResult.json()` is printed to stdout; errors are emitted as JSON on stderr with distinct exit codes.

## Usage
Run the wrapper script from the repository root (or add it to your PATH):

```bash
./micr-cli \
  --image /path/to/check.jpg \
  --format cmc7 \
  --assets /path/to/ultimateMICR-SDK/assets \
  --sdk-root /path/to/ultimateMICR-SDK
```

Optional flags include `--gpgpu-enabled`, `--min-score`, `--threads`, `--no-warmup`, `--no-backprop`, `--no-ielcd`, and `--debug-level`.

## Example output
Running the command above against `assets/images/cmc7_1280x720.jpg` produces a JSON payload similar to the one captured in `sample_output.json` at the repository root.
