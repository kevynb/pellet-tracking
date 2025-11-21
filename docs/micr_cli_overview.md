# MICR CLI Overview

This document describes the architecture and usage of the MICR command-line interface (`micr-cli`) that wraps the Doubango ultimateMICR SDK.

## Goals
- Provide a single-entry CLI to run MICR recognition on static images.
- Keep configuration minimal while exposing common tuning flags (format, GPU toggle, score threshold).
- Ensure results are emitted as JSON on stdout for easy piping or redirection.

## Prerequisites and build steps
- Clone the upstream SDK beside this repository: `git clone https://github.com/DoubangoTelecom/ultimateMICR-SDK.git`.
- Create and activate a virtual environment (optional but recommended) and install Python dependencies: `pip install setuptools Cython numpy pillow`.
- Build the Python extension from `ultimateMICR-SDK/binaries/linux/x86_64` using `python ../../../python/setup.py build_ext --inplace -v`.
- Download a TensorFlow runtime compatible with the SDK (CPU-only is sufficient). The official archive at `https://storage.googleapis.com/tensorflow/libtensorflow/libtensorflow-cpu-linux-x86_64-1.14.0.tar.gz` works; extract it into `ultimateMICR-SDK/binaries/linux/x86_64` so `libtensorflow.so.1` is colocated with `_ultimateMicrSdk.so`.
- Keep the shim directory (`micr_cli_shim`) on `PYTHONPATH` so the legacy `import imp` in the SWIG wrapper works on Python 3.12.

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
