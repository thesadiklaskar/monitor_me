#!/bin/sh
set -eu

VERSION="${MONITOR_AGENT_VERSION-}"
[ -n "$VERSION" ] || VERSION="0.1.0-dev"
case "$VERSION" in
  *[!A-Za-z0-9._+-]*) echo "MONITOR_AGENT_VERSION contains unsupported characters." >&2; exit 1 ;;
esac
OUT="${MONITOR_AGENT_ARTIFACT_DIR-}"
[ -n "$OUT" ] || OUT="/out"
mkdir -p "$OUT"

cd /src/agent
go mod download

build_target() {
  target_os="$1"
  target_arch="$2"
  filename="monitor-agent"
  [ "$target_os" = "windows" ] && filename="monitor-agent.exe"
  directory="$OUT/$target_os-$target_arch"
  mkdir -p "$directory"
  output="$directory/$filename"
  echo "Building $target_os/$target_arch agent..."
  GOOS="$target_os" GOARCH="$target_arch" CGO_ENABLED=0 \
    go build -trimpath -ldflags="-s -w" -o "$output" ./cmd/monitor-agent
  checksum="$(sha256sum "$output" | awk '{print $1}')"
  printf '%s\t%s\t%s\t%s\n' "$target_os" "$target_arch" "$filename" "$checksum" >> "$OUT/.manifest.tsv"
}

rm -f "$OUT/.manifest.tsv" "$OUT/manifest.json"
rm -rf "$OUT/linux-amd64" "$OUT/linux-arm64" "$OUT/windows-amd64"
build_target linux amd64
build_target linux arm64
build_target windows amd64

{
  printf '{\n  "schema_version": 1,\n  "version": "%s",\n  "artifacts": [\n' "$VERSION"
  first=true
  while IFS="$(printf '\t')" read -r target_os target_arch filename checksum; do
    if [ "$first" = true ]; then first=false; else printf ',\n'; fi
    printf '    {"os":"%s","arch":"%s","file":"%s-%s/%s","sha256":"%s"}' \
      "$target_os" "$target_arch" "$target_os" "$target_arch" "$filename" "$checksum"
  done < "$OUT/.manifest.tsv"
  printf '\n  ]\n}\n'
} > "$OUT/manifest.json"
rm -f "$OUT/.manifest.tsv"
chmod -R a+rX "$OUT"
echo "Built native agent bundle version $VERSION in $OUT."
