import { ApiError } from "../api/client";

const maximumAutomaticRetries = 1;
const nonRetryableStatuses = new Set([400, 403, 404, 429]);

export function shouldRetryQuery(failureCount: number, error: Error): boolean {
  if (error instanceof ApiError && nonRetryableStatuses.has(error.status)) {
    return false;
  }
  return failureCount < maximumAutomaticRetries;
}
