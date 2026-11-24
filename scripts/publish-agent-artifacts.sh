#!/bin/sh
set -eu

SOURCE="/bundle"
OUT="${MONITOR_AGENT_ARTIFACT_DIR:-/out}"
STAGING="$OUT/.bundle-staging"

for required in \
  linux-amd64/monitor-agent \
  linux-arm64/monitor-agent \
  windows-amd64/monitor-agent.exe \
  manifest.json; do
  if [ ! -s "$SOURCE/$required" ]; then
    echo "Prebuilt agent artifact is missing: $SOURCE/$required" >&2
    exit 1
  fi
done

mkdir -p "$OUT"
rm -rf "$STAGING"
mkdir -p "$STAGING"
cp -R "$SOURCE/linux-amd64" "$STAGING/linux-amd64"
cp -R "$SOURCE/linux-arm64" "$STAGING/linux-arm64"
cp -R "$SOURCE/windows-amd64" "$STAGING/windows-amd64"
cp "$SOURCE/manifest.json" "$STAGING/manifest.json"

for required in \
  linux-amd64/monitor-agent \
  linux-arm64/monitor-agent \
  windows-amd64/monitor-agent.exe \
  manifest.json; do
  if [ ! -s "$STAGING/$required" ]; then
    echo "Staged agent artifact is incomplete: $STAGING/$required" >&2
    rm -rf "$STAGING"
    exit 1
  fi
done

for platform in linux-amd64 linux-arm64 windows-amd64; do
  rm -rf "$OUT/$platform"
  mv "$STAGING/$platform" "$OUT/$platform"
done
mv "$STAGING/manifest.json" "$OUT/manifest.json"
rm -rf "$STAGING"
chmod -R a+rX "$OUT"

echo "Published prebuilt native agent bundle to $OUT."
