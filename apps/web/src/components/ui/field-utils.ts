export function fieldDescribedBy(
  inputId: string,
  hasDescription: boolean,
  hasError: boolean,
): string | undefined {
  const identifiers = [
    hasDescription ? `${inputId}-description` : "",
    hasError ? `${inputId}-error` : "",
  ].filter(Boolean);
  return identifiers.length > 0 ? identifiers.join(" ") : undefined;
}
