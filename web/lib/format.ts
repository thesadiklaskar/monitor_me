const BYTE_RATE_UNITS = ["KB", "MB", "GB", "TB", "PB"] as const;

/** Format byte throughput using human-readable SI units, starting at KB/s. */
export function formatByteRate(value: number): string {
  if (!Number.isFinite(value)) return "Unavailable";

  const sign = value < 0 ? "-" : "";
  let scaled = Math.abs(value) / 1000;
  let unitIndex = 0;
  while (scaled >= 1000 && unitIndex < BYTE_RATE_UNITS.length - 1) {
    scaled /= 1000;
    unitIndex += 1;
  }

  const precision = scaled < 1 ? 3 : 2;
  return `${sign}${scaled.toFixed(precision)} ${BYTE_RATE_UNITS[unitIndex]}/s`;
}
