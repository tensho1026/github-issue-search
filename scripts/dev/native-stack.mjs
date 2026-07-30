import { existsSync } from "node:fs";
import path from "node:path";
import process from "node:process";

import { NativeProcessGroup } from "./process-group.mjs";

const repositoryRoot = path.resolve(import.meta.dirname, "../..");
const argumentsSet = new Set(process.argv.slice(2));
const supportedArguments = new Set(["--mock", "--smoke"]);
const unknownArguments = [...argumentsSet].filter(
  (argument) => !supportedArguments.has(argument),
);
if (unknownArguments.length > 0) {
  throw new Error(
    `unsupported native stack option: ${unknownArguments.join(", ")}`,
  );
}

loadOptionalEnvironment(path.join(repositoryRoot, "apps/api/.env"));
loadOptionalEnvironment(path.join(repositoryRoot, "apps/web/.env"));

const useMock = argumentsSet.has("--mock");
const smokeOnly = argumentsSet.has("--smoke");
const apiPort = process.env.PORT ?? "8080";
const webPort = process.env.WEB_PORT ?? "5173";
const apiOrigin = `http://127.0.0.1:${apiPort}`;
const webOrigin = `http://127.0.0.1:${webPort}`;
const startupTimeoutMs = parsePositiveInteger(
  "STACK_STARTUP_TIMEOUT_MS",
  process.env.STACK_STARTUP_TIMEOUT_MS ?? "60000",
);
const group = new NativeProcessGroup();
const abortController = new AbortController();
let receivedSignal = "";

for (const signal of ["SIGINT", "SIGTERM"]) {
  process.once(signal, () => {
    receivedSignal = signal;
    abortController.abort(new Error(`received ${signal}`));
  });
}

const apiEnvironment = {
  ...process.env,
  ALLOWED_ORIGINS: process.env.ALLOWED_ORIGINS ?? webOrigin,
  APP_ENV: useMock ? "test" : (process.env.APP_ENV ?? "development"),
  PORT: apiPort,
  USE_GITHUB_API_MOCK: useMock
    ? "true"
    : (process.env.USE_GITHUB_API_MOCK ?? "false"),
};
const webEnvironment = {
  ...process.env,
  VITE_API_BASE_URL: process.env.VITE_API_BASE_URL ?? apiOrigin,
};
const viteEntryPoint = path.join(
  repositoryRoot,
  "apps/web/node_modules/vite/bin/vite.js",
);

try {
  group.start({
    name: "API",
    command: "go",
    args: ["-C", "apps/api", "run", "./cmd/api"],
    cwd: repositoryRoot,
    env: apiEnvironment,
  });
  group.start({
    name: "web",
    command: process.execPath,
    args: [
      viteEntryPoint,
      "--host",
      "127.0.0.1",
      "--port",
      webPort,
      "--strictPort",
    ],
    cwd: path.join(repositoryRoot, "apps/web"),
    env: webEnvironment,
  });

  await Promise.race([
    waitForHealthyStack({
      apiOrigin,
      signal: abortController.signal,
      timeoutMs: startupTimeoutMs,
      webOrigin,
    }),
    group.waitForUnexpectedExit(),
  ]);
  process.stdout.write(
    `[native-stack] ready: web=${webOrigin} api=${apiOrigin}\n`,
  );

  if (smokeOnly) {
    await verifyMockJourney({ apiOrigin, enabled: useMock });
    process.stdout.write(
      "[native-stack] smoke checks passed; stopping services.\n",
    );
  } else {
    await Promise.race([
      waitForAbort(abortController.signal),
      group.waitForUnexpectedExit(),
    ]);
  }
} catch (error) {
  if (!abortController.signal.aborted) {
    process.stderr.write(`[native-stack] ${error.message}\n`);
    process.exitCode = 1;
  }
} finally {
  await group.stop();
}

if (receivedSignal === "SIGINT") {
  process.exitCode = 130;
} else if (receivedSignal === "SIGTERM") {
  process.exitCode = 143;
}

function loadOptionalEnvironment(filename) {
  if (existsSync(filename)) {
    process.loadEnvFile(filename);
  }
}

async function waitForHealthyStack({
  apiOrigin: requestedAPIOrigin,
  signal,
  timeoutMs,
  webOrigin: requestedWebOrigin,
}) {
  const deadline = Date.now() + timeoutMs;
  let lastError = new Error("services have not answered yet");

  while (Date.now() < deadline) {
    signal.throwIfAborted();
    try {
      await Promise.all([
        checkAPIHealth(`${requestedAPIOrigin}/api/health`),
        checkWebHealth(`${requestedWebOrigin}/`),
      ]);
      return;
    } catch (error) {
      lastError = error;
      await delay(250, signal);
    }
  }

  throw new Error(
    `stack did not become ready within ${timeoutMs}ms: ${lastError.message}`,
    { cause: lastError },
  );
}

async function checkAPIHealth(url) {
  const requestID = "native-stack-readiness";
  const response = await fetch(url, {
    headers: { "X-Request-ID": requestID },
    signal: AbortSignal.timeout(1_000),
  });
  const payload = await response.json();
  if (
    !response.ok ||
    payload?.data?.status !== "ok" ||
    payload?.meta?.requestId !== requestID ||
    response.headers.get("x-request-id") !== requestID
  ) {
    throw new Error("API health response or request correlation is invalid");
  }
}

async function checkWebHealth(url) {
  const response = await fetch(url, { signal: AbortSignal.timeout(1_000) });
  const body = await response.text();
  if (!response.ok || !body.includes("<title>IssueScout</title>")) {
    throw new Error("web health response is invalid");
  }
}

async function verifyMockJourney({ apiOrigin: requestedAPIOrigin, enabled }) {
  if (!enabled) {
    return;
  }

  const response = await fetch(
    `${requestedAPIOrigin}/api/github/users/octocat/profile-analysis`,
    { signal: AbortSignal.timeout(5_000) },
  );
  const payload = await response.json();
  if (!response.ok || payload?.data?.username !== "octocat") {
    throw new Error("deterministic profile smoke journey failed");
  }
}

function waitForAbort(signal) {
  if (signal.aborted) {
    return Promise.resolve();
  }
  return new Promise((resolve) => {
    signal.addEventListener("abort", resolve, { once: true });
  });
}

function delay(milliseconds, signal) {
  return new Promise((resolve, reject) => {
    const timeout = setTimeout(resolve, milliseconds);
    signal.addEventListener(
      "abort",
      () => {
        clearTimeout(timeout);
        reject(signal.reason);
      },
      { once: true },
    );
  });
}

function parsePositiveInteger(name, value) {
  if (!/^[1-9]\d*$/.test(value)) {
    throw new Error(`${name} must be a positive integer`);
  }
  return Number(value);
}
