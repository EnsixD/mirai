#!/bin/sh
# Bootstrap verifies the release signature and installer hash before running Python.
set -eu
[ "$(id -u)" = 0 ] || { echo 'Run as root'; exit 1; }
task_dir=$(mktemp -d)
trap 'rm -rf "$task_dir"' EXIT HUP INT TERM
base=https://github.com/EnsixD/mirai/releases/latest/download
curl -fsSL "$base/manifest.json" -o "$task_dir/manifest.json"
curl -fsSL "$base/manifest.json.sig" -o "$task_dir/signature.b64"
printf '%s\n' '-----BEGIN PUBLIC KEY-----' 'MCowBQYDK2VwAyEAOCOOJzn+R2XICC8i5jCInj4vwKyrSEa5PN5eigk8Mcg=' '-----END PUBLIC KEY-----' > "$task_dir/public.pem"
base64 -d "$task_dir/signature.b64" > "$task_dir/signature"
openssl pkeyutl -verify -pubin -inkey "$task_dir/public.pem" -rawin -in "$task_dir/manifest.json" -sigfile "$task_dir/signature"
curl -fsSL "$base/install.py" -o "$task_dir/install.py"
python3 - "$task_dir" <<'PY'
import hashlib,json,sys
from pathlib import Path
p=Path(sys.argv[1])
m=json.loads((p/'manifest.json').read_bytes())
if hashlib.sha256((p/'install.py').read_bytes()).hexdigest()!=m['files']['install.py']:
    raise SystemExit('Installer checksum mismatch')
PY
python3 "$task_dir/install.py" "$@"
