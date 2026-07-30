import { describe, expect, it } from "vitest";

import { ApiError } from "../../../shared/api/client";
import {
  prioritizedProfileError,
  profileErrorPresentation,
} from "./profile-error";

describe("profile error presentation", () => {
  it("makes not-found and rate-limit failures recoverable", () => {
    const notFound = profileErrorPresentation(
      new ApiError({
        code: "GITHUB_USER_NOT_FOUND",
        message: "not found",
        requestId: "req_404",
        status: 404,
      }),
      "missing",
    );
    expect(notFound).toMatchObject({
      requestId: "req_404",
      retryable: false,
      title: "Profile not found",
    });

    const rateLimit = profileErrorPresentation(
      new ApiError({
        code: "GITHUB_RATE_LIMIT_EXCEEDED",
        message: "limited",
        status: 429,
      }),
      "octocat",
    );
    expect(rateLimit).toMatchObject({
      retryable: true,
      title: "GitHub needs a breather",
    });
  });

  it("prioritizes actionable rate limits across parallel requests", () => {
    const upstream = new ApiError({
      code: "GITHUB_API_ERROR",
      message: "upstream",
      status: 502,
    });
    const limited = new ApiError({
      code: "GITHUB_RATE_LIMIT_EXCEEDED",
      message: "limited",
      status: 429,
    });
    expect(prioritizedProfileError([upstream, limited])).toBe(limited);
  });
});
