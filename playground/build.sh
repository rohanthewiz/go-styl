#!/bin/sh
# Builds the WASM playground into this directory:
#   playground/styl.wasm     (compiler, built js/wasm)
#   playground/wasm_exec.js  (Go's JS support shim, copied from GOROOT)
# Serve the directory with any static file server, e.g.:
#   go run ./playground/serve       # tiny bundled server on :8080
set -eu
cd "$(dirname "$0")"

GOOS=js GOARCH=wasm go build -trimpath -ldflags='-s -w' -o styl.wasm ./wasm

goroot=$(go env GOROOT)
if [ -f "$goroot/lib/wasm/wasm_exec.js" ]; then     # Go >= 1.24
  cp "$goroot/lib/wasm/wasm_exec.js" .
else                                                # Go <= 1.23
  cp "$goroot/misc/wasm/wasm_exec.js" .
fi

ls -lh styl.wasm
