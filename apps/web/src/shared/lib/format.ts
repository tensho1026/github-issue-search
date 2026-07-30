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

export function formatPercentage(value: number): string {
  return `${Math.max(0, Math.min(100, Math.round(value)))}%`;
}
