import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import path from "node:path";
import process from "node:process";
import test from "node:test";

const repositoryRoot = path.resolve(import.meta.dirname, "../..");
const stackEntryPoint = path.join(import.meta.dirname, "native-stack.mjs");
const apiOrigin = "http://127.0.0.1:18181";
const webOrigin = "http://127.0.0.1:15174";

test("SIGTERM after readiness cleans up the complete native stack", async (t) => {
  const child = spawn(process.execPath, [stackEntryPoint, "--mock"], {
    cwd: repositoryRoot,
    env: {
      ...process.env,
      ALLOWED_ORIGINS: webOrigin,
      PORT: "18181",
      STACK_STARTUP_TIMEOUT_MS: "20000",
      VITE_API_BASE_URL: apiOrigin,
      WEB_PORT: "15174",
    },
    stdio: ["ignore", "pipe", "pipe"],
  });
  let output = "";
  child.stdout.on("data", (chunk) => {
    output += chunk;
  });
  child.stderr.on("data", (chunk) => {
    output += chunk;
  });
  t.after(() => {
    if (child.exitCode === null && child.signalCode === null) {
      child.kill("SIGKILL");
    }
  });

  await waitUntil(() => output.includes("[native-stack] ready:"), 20_000);
  assert.equal((await fetch(`${apiOrigin}/api/health`)).status, 200);
  assert.equal((await fetch(webOrigin)).status, 200);

  child.kill("SIGTERM");
  const result = await waitForExit(child, 15_000);

  assert.equal(result.code, 143, output);
  await assertEndpointStopped(`${apiOrigin}/api/health`);
  await assertEndpointStopped(webOrigin);
});

async function assertEndpointStopped(url) {
  await assert.rejects(
    fetch(url, { signal: AbortSignal.timeout(500) }),
    /fetch failed|aborted|timeout/i,
  );
}

function waitForExit(child, timeoutMs) {
  return new Promise((resolve, reject) => {
    const timeout = setTimeout(() => {
      reject(new Error("native stack did not exit after SIGTERM"));
    }, timeoutMs);
    child.once("exit", (code, signal) => {
      clearTimeout(timeout);
      resolve({ code, signal });
    });
  });
}

async function waitUntil(predicate, timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (predicate()) {
      return;
    }
    await new Promise((resolve) => setTimeout(resolve, 25));
  }
  throw new Error("native stack did not report readiness before the deadline");
}
