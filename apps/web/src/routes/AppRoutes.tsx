import { lazy, Suspense } from "react";
import { BrowserRouter, Route, Routes } from "react-router";

import { AppShell } from "../components/layout/AppShell";
import { Skeleton } from "../components/ui/skeleton";
import { appRoutes } from "../shared/config/app-config";

const HomePage = lazy(async () => {
  const module = await import("../pages/HomePage");
  return { default: module.HomePage };
});

function RouteFallback() {
  return (
    <div
      aria-label="Loading page"
      className="mx-auto grid min-h-[60vh] w-full max-w-7xl content-center gap-5 px-5 sm:px-8 lg:px-10"
      role="status"
    >
      <Skeleton className="h-6 w-32" />
      <Skeleton className="h-16 w-full max-w-2xl" />
      <Skeleton className="h-32 w-full max-w-3xl" />
    </div>
  );
}

export function AppRoutes() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<AppShell />}>
          <Route
            element={
              <Suspense fallback={<RouteFallback />}>
                <HomePage />
              </Suspense>
            }
            path={appRoutes.home}
          />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
