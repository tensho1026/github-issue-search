import assert from "node:assert/strict";
import { mkdtemp, readFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import process from "node:process";
import test from "node:test";

import { NativeProcessGroup } from "./process-group.mjs";

const fixture = path.resolve(
  import.meta.dirname,
  "fixtures/long-lived-child.mjs",
);

test("stops every child when startup is interrupted", async () => {
  const temporaryDirectory = await mkdtemp(
    path.join(tmpdir(), "issuescout-native-startup-"),
  );
  const group = new NativeProcessGroup();
  const children = startFixtures(group, temporaryDirectory, 2);

  await group.stop({ timeoutMs: 2_000 });

  await assertAllStopped(children);
});

test("stops every ready child gracefully", async () => {
  const temporaryDirectory = await mkdtemp(
    path.join(tmpdir(), "issuescout-native-ready-"),
  );
  const group = new NativeProcessGroup();
  const children = startFixtures(group, temporaryDirectory, 2);
  await Promise.all(children.map(({ pidFile }) => waitForFile(pidFile)));

  await group.stop({ timeoutMs: 2_000 });

  await assertAllStopped(children);
  for (const { stoppedFile } of children) {
    assert.match(await readFile(stoppedFile, "utf8"), /:SIGTERM\n$/);
  }
});

function startFixtures(group, temporaryDirectory, count) {
  return Array.from({ length: count }, (_, index) => {
    const pidFile = path.join(temporaryDirectory, `${index}.pid`);
    const stoppedFile = path.join(temporaryDirectory, `${index}.stopped`);
    const child = group.start({
      name: `fixture-${index}`,
      command: process.execPath,
      args: [fixture, pidFile, stoppedFile],
    });
    return { child, pidFile, stoppedFile };
  });
}

async function assertAllStopped(children) {
  for (const { child } of children) {
    assert.ok(child.exitCode !== null || child.signalCode !== null);
    assert.throws(() => process.kill(child.pid, 0), { code: "ESRCH" });
  }
}

async function waitForFile(filename) {
  for (let attempt = 0; attempt < 100; attempt += 1) {
    try {
      await readFile(filename, "utf8");
      return;
    } catch (error) {
      if (error?.code !== "ENOENT") {
        throw error;
      }
      await new Promise((resolve) => setTimeout(resolve, 10));
    }
  }
  throw new Error(`fixture did not become ready: ${filename}`);
}
