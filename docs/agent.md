# Agent Handoff Notes

This file captures the current state of the MICR CLI work so the next contributor can continue smoothly.

## Repository state
- The CLI wrapper (`micr_cli.py`) and shell entrypoint (`micr-cli`) are present at repo root; both expect the upstream `ultimateMICR-SDK` to be cloned beside this repository (ignored via `.gitignore`).
- A Python shim (`micr_cli_shim/imp.py`) supplies the deprecated `imp` module needed by the SWIG-generated `ultimateMicrSdk.py` on Python 3.12+.
- Real sample output from running the CLI lives in `sample_output.json`.
- Documentation lives in `docs/micr_cli_overview.md` (how it works) and `docs/micr_cli_issue_log.md` (problems solved during setup).

## Verified runtime path
1. Build the Python extension from `ultimateMICR-SDK/binaries/linux/x86_64` with `python ../../../python/setup.py build_ext --inplace -v` after installing `setuptools`, `Cython`, `numpy`, and `pillow` in your environment.
2. Download TensorFlow CPU runtime `libtensorflow-cpu-linux-x86_64-1.14.0.tar.gz` and place `libtensorflow.so.1` (and symlinks) into `ultimateMICR-SDK/binaries/linux/x86_64` so it sits next to `_ultimateMicrSdk.so`.
3. Run the CLI from repo root, e.g.:
   ```bash
   ./micr-cli \
     --image ultimateMICR-SDK/assets/images/cmc7_1280x720.jpg \
     --format cmc7 \
     --assets ultimateMICR-SDK/assets \
     --sdk-root ultimateMICR-SDK
   ```
   This prints JSON to stdout; see `sample_output.json` for a captured example.

## Known pitfalls
- The upstream SWIG wrapper imports `imp`; ensure `micr_cli_shim` stays on `PYTHONPATH` (handled automatically by `micr_cli.py`).
- If imports fail with `libultimate_micr-sdk.so` or `libtensorflow.so.1` missing, confirm `LD_LIBRARY_PATH` includes `ultimateMICR-SDK/binaries/linux/x86_64` or rely on the CLI, which preloads these via `ctypes`.
- Network access to Doubango’s TensorFlow mirror failed earlier (HTTP 403); using the official TensorFlow CPU tarball succeeded.

## Outstanding questions / next steps
- Consider packaging the CLI (and shim) into a virtual environment or container for reproducibility; currently relies on environment variables.
- Evaluate GPU and threading options for performance once hardware support is available.
- Add automated smoke tests around `micr_cli.py` if the SDK license allows shipping the binary assets in CI.
