import { appConfig } from "../config/app-config";
import type { ErrorEnvelope } from "./generated";

type ApiErrorOptions = {
  code: string;
  message: string;
  requestId?: string;
  status: number;
};

export class ApiError extends Error {
  readonly code: string;
  readonly requestId?: string;
  readonly status: number;

  constructor(options: ApiErrorOptions) {
    super(options.message);
    this.name = "ApiError";
    this.code = options.code;
    this.requestId = options.requestId;
    this.status = options.status;
  }
}

type ApiRequestOptions = Omit<RequestInit, "body" | "method"> & {
  method?: "GET";
};

export interface ApiClient {
  get<TResponse>(
    path: `/${string}`,
    options?: ApiRequestOptions,
  ): Promise<TResponse>;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function isErrorEnvelope(value: unknown): value is ErrorEnvelope {
  if (!isRecord(value) || !isRecord(value.error) || !isRecord(value.meta)) {
    return false;
  }
  return (
    typeof value.error.code === "string" &&
    typeof value.error.message === "string" &&
    typeof value.meta.requestId === "string"
  );
}

async function readJson(response: Response): Promise<unknown> {
  const contentType = response.headers.get("content-type") ?? "";
  if (!contentType.toLowerCase().startsWith("application/json")) {
    return undefined;
  }
  try {
    return await response.json();
  } catch {
    return undefined;
  }
}

function requestUrl(path: `/${string}`): string {
  return `${appConfig.apiBaseUrl}${path}`;
}

export function createApiClient(
  request: typeof fetch = (...arguments_) => globalThis.fetch(...arguments_),
): ApiClient {
  return {
    async get<TResponse>(
      path: `/${string}`,
      options: ApiRequestOptions = {},
    ): Promise<TResponse> {
      const response = await request(requestUrl(path), {
        ...options,
        headers: {
          Accept: "application/json",
          ...options.headers,
        },
        method: "GET",
      });
      const payload = await readJson(response);

      if (!response.ok) {
        if (isErrorEnvelope(payload)) {
          throw new ApiError({
            code: payload.error.code,
            message: payload.error.message,
            requestId: payload.meta.requestId,
            status: response.status,
          });
        }
        throw new ApiError({
          code: "INVALID_ERROR_RESPONSE",
          message: "IssueScout received an invalid API error response.",
          status: response.status,
        });
      }
      if (payload === undefined) {
        throw new ApiError({
          code: "INVALID_SUCCESS_RESPONSE",
          message: "IssueScout received an invalid API success response.",
          status: 502,
        });
      }

      return payload as TResponse;
    },
  };
}

export const apiClient = createApiClient();
