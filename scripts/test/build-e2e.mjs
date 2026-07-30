import { spawnSync } from "node:child_process";
import process from "node:process";

const pnpm = process.platform === "win32" ? "pnpm.cmd" : "pnpm";
const apiBaseURL = "http://127.0.0.1:18080";
const result = spawnSync(pnpm, ["--filter", "@issuescout/web", "build"], {
  env: {
    ...process.env,
    VITE_API_BASE_URL: apiBaseURL,
  },
  stdio: "inherit",
});

if (result.error) {
  throw result.error;
}
if (result.status !== 0) {
  process.exit(result.status ?? 1);
}
