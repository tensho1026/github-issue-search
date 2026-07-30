import { describe, expect, it, vi } from "vitest";

import { createApiClient } from "./client";

describe("createApiClient", () => {
  it("passes cancellation and returns a typed JSON response", async () => {
    const controller = new AbortController();
    const request = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(JSON.stringify({ data: { status: "ok" } }), {
        headers: { "Content-Type": "application/json" },
        status: 200,
      }),
    );
    const client = createApiClient(request);

    await expect(
      client.get<{ data: { status: string } }>("/api/health", {
        signal: controller.signal,
      }),
    ).resolves.toEqual({ data: { status: "ok" } });
    expect(request).toHaveBeenCalledWith(
      "/api/health",
      expect.objectContaining({
        headers: { Accept: "application/json" },
        method: "GET",
        signal: controller.signal,
      }),
    );
  });

  it("normalizes the shared API error envelope", async () => {
    const request = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(
        JSON.stringify({
          error: {
            code: "GITHUB_USER_NOT_FOUND",
            message: "GitHub user was not found",
          },
          meta: {
            requestId: "req_test",
            timestamp: "2026-07-30T00:00:00Z",
          },
        }),
        {
          headers: { "Content-Type": "application/json" },
          status: 404,
        },
      ),
    );

    const promise = createApiClient(request).get("/api/missing");

    await expect(promise).rejects.toMatchObject({
      code: "GITHUB_USER_NOT_FOUND",
      requestId: "req_test",
      status: 404,
    });
  });

  it("rejects successful non-JSON responses", async () => {
    const request = vi
      .fn<typeof fetch>()
      .mockResolvedValue(new Response("ok", { status: 200 }));

    await expect(
      createApiClient(request).get("/api/health"),
    ).rejects.toMatchObject({
      code: "INVALID_SUCCESS_RESPONSE",
      status: 502,
    });
  });
});
