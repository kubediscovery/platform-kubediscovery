#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROTO_DIR="$ROOT_DIR/proto/kubediscovery/v1"
OUT_DIR="$ROOT_DIR/libs"
MODULE_PATH="github.com/kubediscovery/kd-libs"

require_bin() {
  local bin="$1"
  if ! command -v "$bin" >/dev/null 2>&1; then
    echo "error: required binary '$bin' not found in PATH" >&2
    exit 1
  fi
}

require_bin protoc
require_bin protoc-gen-go
require_bin protoc-gen-go-grpc

if [[ ! -d "$PROTO_DIR" ]]; then
  echo "error: proto directory not found: $PROTO_DIR" >&2
  exit 1
fi

mapfile -t PROTO_FILES < <(find "$PROTO_DIR" -maxdepth 1 -type f -name '*.proto' | sort)

if [[ ${#PROTO_FILES[@]} -eq 0 ]]; then
  echo "error: no .proto files found in $PROTO_DIR" >&2
  exit 1
fi

echo "Generating Go protobuf files from ${#PROTO_FILES[@]} proto(s)..."

protoc \
  -I "$ROOT_DIR/proto" \
  --go_out="$OUT_DIR" \
  --go_opt=module="$MODULE_PATH" \
  --go-grpc_out="$OUT_DIR" \
  --go-grpc_opt=module="$MODULE_PATH" \
  "${PROTO_FILES[@]}"

echo "Done. Generated files under libs/core/v1/."
