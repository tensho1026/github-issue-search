import { BrowserRouter, Route, Routes } from "react-router";

import { AppShell } from "../components/layout/AppShell";
import { HomePage } from "../pages/HomePage";
import { appRoutes } from "../shared/config/app-config";

export function AppRoutes() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<AppShell />}>
          <Route element={<HomePage />} path={appRoutes.home} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
