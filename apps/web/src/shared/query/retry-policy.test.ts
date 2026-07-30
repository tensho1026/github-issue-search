import { describe, expect, it } from "vitest";

import { ApiError } from "../api/client";
import { shouldRetryQuery } from "./retry-policy";

describe("shouldRetryQuery", () => {
  it.each([400, 403, 404, 429])("does not retry status %d", (status) => {
    expect(
      shouldRetryQuery(
        0,
        new ApiError({ code: "TEST", message: "test", status }),
      ),
    ).toBe(false);
  });

  it("retries one transient failure only", () => {
    expect(shouldRetryQuery(0, new Error("network"))).toBe(true);
    expect(shouldRetryQuery(1, new Error("network"))).toBe(false);
  });
});
