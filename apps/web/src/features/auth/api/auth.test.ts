import { afterEach, describe, expect, it, vi } from "vitest";

import { logoutAuthSession, readAuthSession, refreshAuthSession } from "./auth";

function response() {
  return Promise.resolve(
    new Response(
      JSON.stringify({
        data: { authenticated: false, configured: true },
        meta: { requestId: "req", timestamp: "2026-08-01T00:00:00Z" },
      }),
      { headers: { "Content-Type": "application/json" }, status: 200 },
    ),
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("auth API", () => {
  it("hydrates and mutates cookie sessions with in-memory CSRF", async () => {
    const request = vi.fn<typeof fetch>().mockImplementation(response);
    vi.stubGlobal("fetch", request);
    const signal = new AbortController().signal;

    await readAuthSession(signal);
    await refreshAuthSession("csrf");
    await logoutAuthSession("csrf");

    expect(request.mock.calls.map(([path]) => path)).toEqual([
      "/api/auth/session",
      "/api/auth/session/refresh",
      "/api/auth/logout",
    ]);
    expect(request.mock.calls[0]?.[1]).toMatchObject({
      credentials: "include",
      method: "GET",
      signal,
    });
    for (const index of [1, 2]) {
      expect(request.mock.calls[index]?.[1]).toMatchObject({
        body: "{}",
        credentials: "include",
        method: "POST",
      });
      expect(
        new Headers(request.mock.calls[index]?.[1]?.headers).get(
          "X-CSRF-Token",
        ),
      ).toBe("csrf");
    }
  });
});
