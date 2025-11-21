# MICR CLI Build Issues and Resolutions

## 2025-11-20 (session)
 - **Issue:** Upstream `ultimateMICR-SDK` repository contains large binaries that should not be committed to this project.
 - **Resolution:** Do not vendor the SDK; fetch a shallow clone on demand so the sources are reproducible without inflating this repo.

- **Issue:** The Python setup script depends on `distutils`, which is absent in the base image's Python installation, causing `ModuleNotFoundError: No module named 'distutils'` when building the extension.
- **Resolution:** The distro packages for `distutils` are not available, so the fix was to install `setuptools`, which vendors `distutils` at `setuptools._distutils` and satisfies the import expected by the SDK's `setup.py`.

- **Issue:** Building the Python extension fails with `ModuleNotFoundError: No module named 'Cython'` because the setup script expects Cython to be preinstalled.
- **Resolution:** Installed `Cython` into the virtual environment via `pip install Cython` before rerunning the build.

- **Issue:** Downloading the TensorFlow runtime from Doubango's mirror (`doubango.org`) failed with HTTP 403 errors.
- **Resolution:** Switched to the official TensorFlow CPU runtime (`https://storage.googleapis.com/tensorflow/libtensorflow/libtensorflow-cpu-linux-x86_64-1.14.0.tar.gz`), extracted it, and moved the shared libraries into `ultimateMICR-SDK/binaries/linux/x86_64`.

- **Issue:** The SWIG-generated `ultimateMicrSdk.py` relies on the deprecated `imp` module, which is removed in Python 3.12, causing imports to fail.
- **Resolution:** Added a lightweight shim (`micr_cli_shim/imp.py`) and prepended its path to `PYTHONPATH` so the wrapper can still import successfully on Python 3.12+.

- **Issue:** Importing the Python bindings from the new CLI failed with `libultimate_micr-sdk.so` not found even after adjusting `LD_LIBRARY_PATH`.
- **Resolution:** Explicitly preload `libultimate_micr-sdk.so` and `libtensorflow.so.1` via `ctypes.CDLL` inside `micr_cli.py` before importing the SWIG wrapper.

## 2025-11-21 (session)
- **Issue:** Setup required multiple manual steps (clone SDK, install build deps, fetch TensorFlow, build extension), making onboarding slow and error-prone.
- **Resolution:** Added a single entry-point script (`scripts/setup_micr.sh`) that uses `uv` to create `.venv`, install dependencies, pull down the `ultimateMICR-SDK` sources with depth 1, download TensorFlow automatically, and compile the Python extension.

- **Issue:** Developers without `uv` installed could not use the simplified flow.
- **Resolution:** The setup script now bootstraps `uv` from the official installer when missing and falls back to the system Python if the venv is absent when running `micr-cli`.

## 2025-11-22 (session)
- **Issue:** Tracking `ultimateMICR-SDK` as a git submodule caused PR update failures and bloated metadata.
- **Resolution:** Removed the submodule from version control and updated `scripts/setup_micr.sh` to clone or refresh a shallow (`--depth 1`) checkout locally on demand.
