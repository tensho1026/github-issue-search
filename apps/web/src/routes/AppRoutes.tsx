import { lazy, Suspense, type ReactNode } from "react";
import { BrowserRouter, Route, Routes } from "react-router";

import { AppShell } from "../components/layout/AppShell";
import { Skeleton } from "../components/ui/skeleton";
import { appRoutes } from "../shared/config/app-config";

const HomePage = lazy(async () => {
  const module = await import("../pages/HomePage");
  return { default: module.HomePage };
});

const ProfilePage = lazy(async () => {
  const module = await import("../pages/ProfilePage");
  return { default: module.ProfilePage };
});

const IssueSearchPage = lazy(async () => {
  const module = await import("../pages/IssueSearchPage");
  return { default: module.IssueSearchPage };
});

const NotFoundPage = lazy(async () => {
  const module = await import("../pages/NotFoundPage");
  return { default: module.NotFoundPage };
});

function LazyPage({ children }: { children: ReactNode }) {
  return <Suspense fallback={<RouteFallback />}>{children}</Suspense>;
}

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
              <LazyPage>
                <HomePage />
              </LazyPage>
            }
            path={appRoutes.home}
          />
          <Route
            element={
              <LazyPage>
                <ProfilePage />
              </LazyPage>
            }
            path={appRoutes.profilePattern}
          />
          <Route
            element={
              <LazyPage>
                <IssueSearchPage />
              </LazyPage>
            }
            path={appRoutes.search}
          />
          <Route
            element={
              <LazyPage>
                <NotFoundPage />
              </LazyPage>
            }
            path="*"
          />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
