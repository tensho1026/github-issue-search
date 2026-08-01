import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useLocation, MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";

import { AppProviders } from "../../../app/AppProviders";
import { AccountControl } from "./AccountControl";
import { AuthFeedback } from "./AuthFeedback";

const meta = {
  requestId: "req_auth",
  timestamp: "2026-08-01T00:00:00Z",
};

function jsonResponse(payload: unknown, status = 200) {
  return Promise.resolve(
    new Response(JSON.stringify(payload), {
      headers: { "Content-Type": "application/json" },
      status,
    }),
  );
}

function requestUrl(input: RequestInfo | URL): string {
  if (typeof input === "string") {
    return input;
  }
  return input instanceof URL ? input.href : input.url;
}

function authenticated() {
  return {
    data: {
      authenticated: true,
      configured: true,
      csrfToken: "csrf-memory-only",
      user: {
        accountId: "00000000-0000-4000-8000-000000000001",
        avatarUrl: "https://avatars.githubusercontent.com/u/1",
        login: "octocat",
        profileUrl: "https://github.com/octocat",
      },
    },
    meta,
  };
}

function LocationProbe() {
  const location = useLocation();
  return <output aria-label="Current route">{location.search}</output>;
}

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("AccountControl", () => {
  it("hydrates an optional anonymous session without blocking public UI", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockImplementation(() =>
        jsonResponse({
          data: { authenticated: false, configured: true },
          meta,
        }),
      ),
    );

    render(
      <AppProviders>
        <MemoryRouter>
          <AccountControl />
        </MemoryRouter>
      </AppProviders>,
    );

    expect(
      await screen.findByRole("button", { name: "Sign in" }),
    ).toBeInTheDocument();
  });

  it("revokes the server session with CSRF before showing anonymous state", async () => {
    const request = vi.fn<typeof fetch>().mockImplementation((input) => {
      return requestUrl(input) === "/api/auth/session"
        ? jsonResponse(authenticated())
        : jsonResponse({ data: { loggedOut: true }, meta });
    });
    vi.stubGlobal("fetch", request);
    const user = userEvent.setup();

    render(
      <AppProviders>
        <MemoryRouter>
          <AccountControl />
        </MemoryRouter>
      </AppProviders>,
    );

    await user.click(
      await screen.findByRole("button", {
        name: "Open account menu for octocat",
      }),
    );
    await user.click(screen.getByRole("button", { name: "Sign out" }));

    expect(
      await screen.findByRole("button", { name: "Sign in" }),
    ).toBeInTheDocument();
    const logout = request.mock.calls.find(
      ([path]) => path === "/api/auth/logout",
    );
    expect(logout?.[1]).toMatchObject({
      credentials: "include",
      method: "POST",
    });
    expect(new Headers(logout?.[1]?.headers).get("X-CSRF-Token")).toBe(
      "csrf-memory-only",
    );
  });
});

describe("AuthFeedback", () => {
  it("hydrates a successful callback and removes its URL marker", async () => {
    const request = vi
      .fn<typeof fetch>()
      .mockImplementation(() => jsonResponse(authenticated()));
    vi.stubGlobal("fetch", request);

    render(
      <AppProviders>
        <MemoryRouter
          initialEntries={["/search?username=octocat&auth=success"]}
        >
          <AuthFeedback />
          <LocationProbe />
        </MemoryRouter>
      </AppProviders>,
    );

    expect(
      await screen.findByText("Signed in successfully"),
    ).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByLabelText("Current route")).toHaveTextContent(
        "?username=octocat",
      );
    });
    expect(request).toHaveBeenCalledTimes(1);
  });

  it.each([
    ["denied", "Sign-in cancelled"],
    ["error", "Sign-in was not completed"],
  ])("renders the safe %s callback state", (marker, title) => {
    const request = vi.fn<typeof fetch>();
    vi.stubGlobal("fetch", request);

    render(
      <AppProviders>
        <MemoryRouter initialEntries={[`/search?auth=${marker}`]}>
          <AuthFeedback />
        </MemoryRouter>
      </AppProviders>,
    );

    expect(screen.getByText(title)).toBeInTheDocument();
    expect(request).not.toHaveBeenCalled();
  });
});
