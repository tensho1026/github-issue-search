const compactNumberFormatter = new Intl.NumberFormat("en", {
  maximumFractionDigits: 1,
  notation: "compact",
});

const dateFormatter = new Intl.DateTimeFormat("en-GB", {
  day: "numeric",
  month: "short",
  year: "numeric",
});

export function formatCompactNumber(value: number): string {
  return compactNumberFormatter.format(Math.max(0, value));
}

export function formatDate(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "Unknown" : dateFormatter.format(date);
}

export function formatDuration(seconds: number | null): string {
  if (seconds === null || !Number.isFinite(seconds) || seconds < 0) {
    return "Unavailable";
  }
  if (seconds < 60) {
    return "< 1 min";
  }
  if (seconds < 3600) {
    return `${Math.round(seconds / 60)} min`;
  }
  if (seconds < 172_800) {
    const hours = Math.round(seconds / 3600);
    return `${hours} ${hours === 1 ? "hr" : "hrs"}`;
  }
  const days = Math.round(seconds / 86_400);
  return `${days} days`;
}

export function formatPercentage(value: number): string {
  return `${Math.max(0, Math.min(100, Math.round(value)))}%`;
}
