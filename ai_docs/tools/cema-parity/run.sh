#!/usr/bin/env bash
# Compile cema's stylesheets with go-styl and reference stylus, then compare
# them semantically (cmp.py) and by rule/selector order.
#
# Usage: run.sh <workdir>
#   <workdir>/styl  a copy of cema's styles/styl (patch cema-side quirks there)
#   writes <workdir>/out/<sheet>.{go,stylus}.css, .diff, .order
# Needs: stylus 0.64 on PATH, go, python3. Run from the go-styl repo root.
set -euo pipefail
W=$(cd "$1" && pwd)
HERE=$(cd "$(dirname "$0")" && pwd)
go build -o "$W/styl-bin" ./cmd/styl
mkdir -p "$W/out"
cd "$W/styl"
for f in master.styl theme_masters/*.styl; do
  n=$(basename "$f" .styl)
  stylus -p "$f" > "$W/out/$n.stylus.css"
  "$W/styl-bin" "$f" > "$W/out/$n.go.css"
done
cd "$W/out"
for g in *.go.css; do
  n=${g%.go.css}
  python3 "$HERE/cmp.py" "$n.go.css" "$n.stylus.css" > "$n.diff" || true
  tail -1 "$n.diff"
  for k in go stylus; do
    python3 - "$n.$k.css" > "$n.$k.order" <<'PY'
import re, sys
css = re.sub(r'/\*.*?\*/', '', open(sys.argv[1]).read(), flags=re.S)
for h in re.findall(r'([^{};]+)\{', css):
    h = re.sub(r'\s+', ' ', h.strip())
    print(re.sub(r'\s*([>+~,])\s*', r'\1', h).replace("'", '"'))
PY
  done
  cmp -s "$n.go.order" "$n.stylus.order" && echo "   same rule and selector order" || echo "   ORDER DIFFERS (diff $n.go.order $n.stylus.order)"
done
