# MICR CLI Build Issues and Resolutions

## 2025-11-20 (session)
- **Issue:** Upstream `ultimateMICR-SDK` repository contains large binaries that should not be committed to this project.
- **Resolution:** Added `ultimateMICR-SDK/` and `micr_venv/` to `.gitignore` to keep the local clone and virtual environment out of version control while still allowing us to build and run the CLI.

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
