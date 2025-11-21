#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SDK_DIR="$ROOT_DIR/ultimateMICR-SDK"
BIN_DIR="$SDK_DIR/binaries/linux/x86_64"
VENV_DIR="${VENV_DIR:-$ROOT_DIR/.venv}"
TENSORFLOW_URL="${TENSORFLOW_URL:-https://storage.googleapis.com/tensorflow/libtensorflow/libtensorflow-cpu-linux-x86_64-1.14.0.tar.gz}"

log() {
  echo "[setup] $*"
}

ensure_uv() {
  if command -v uv >/dev/null 2>&1; then
    return
  fi
  log "uv not found; installing via https://astral.sh/uv/install.sh"
  curl -LsSf https://astral.sh/uv/install.sh | sh
  export PATH="$HOME/.local/bin:$PATH"
  if ! command -v uv >/dev/null 2>&1; then
    echo "uv installation failed" >&2
    exit 1
  fi
}

ensure_sdk_repo() {
  if [ -d "$SDK_DIR/.git" ]; then
    log "Updating existing ultimateMICR-SDK clone (depth=1)"
    git -C "$SDK_DIR" fetch --depth 1 origin main
    git -C "$SDK_DIR" reset --hard FETCH_HEAD
    return
  fi

  if [ -e "$SDK_DIR" ]; then
    log "Removing non-git SDK directory at $SDK_DIR"
    rm -rf "$SDK_DIR"
  fi

  log "Cloning ultimateMICR-SDK (depth=1)"
  git clone --depth 1 https://github.com/DoubangoTelecom/ultimateMICR-SDK.git "$SDK_DIR"
}

create_venv() {
  ensure_uv
  log "Creating virtual environment at $VENV_DIR"
  uv venv "$VENV_DIR"
}

install_python_dependencies() {
  log "Installing Python dependencies via uv pip"
  uv pip install --python "$VENV_DIR/bin/python" -r "$ROOT_DIR/scripts/requirements-micr.txt"
}

build_python_extension() {
  log "Building ultimateMICR Python extension"
  pushd "$BIN_DIR" >/dev/null
  "$VENV_DIR/bin/python" ../../../python/setup.py build_ext --inplace -v
  popd >/dev/null
}

download_tensorflow() {
  if [ -f "$BIN_DIR/libtensorflow.so.1" ]; then
    log "TensorFlow runtime already present; skipping download"
    return
  fi

  log "Downloading TensorFlow runtime from $TENSORFLOW_URL"
  tmp_dir="$(mktemp -d)"
  curl -L "$TENSORFLOW_URL" -o "$tmp_dir/libtensorflow.tar.gz"
  tar -xzf "$tmp_dir/libtensorflow.tar.gz" -C "$tmp_dir"
  find "$tmp_dir" -maxdepth 3 -type f -name "libtensorflow*.so*" -print0 | while IFS= read -r -d '' file; do
    log "Copying $(basename "$file")"
    cp "$file" "$BIN_DIR/"
  done
  for base in libtensorflow libtensorflow_framework; do
    if [ -f "$BIN_DIR/${base}.so.1.14.0" ]; then
      ln -sf "${base}.so.1.14.0" "$BIN_DIR/${base}.so.1"
      ln -sf "${base}.so.1.14.0" "$BIN_DIR/${base}.so"
    fi
  done
  rm -rf "$tmp_dir"
}

main() {
  ensure_sdk_repo
  create_venv
  install_python_dependencies
  download_tensorflow
  build_python_extension
  log "Setup complete. Activate the venv with: source $VENV_DIR/bin/activate"
}

main "$@"
