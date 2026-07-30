import { execFileSync } from "node:child_process";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import process from "node:process";

const [version, referenceDirectoryArgument] = process.argv.slice(2);
if (!/^v\d+\.\d+\.\d+(?:[+-][0-9A-Za-z.-]+)?$/.test(version ?? "")) {
  throw new Error(
    "usage: check-reproducibility.mjs <semantic-version> [reference-directory]",
  );
}

const repositoryRoot = path.resolve(import.meta.dirname, "../..");
const temporaryRoot = await mkdtemp(
  path.join(tmpdir(), "issuescout-reproducibility-"),
);
const firstDirectory = referenceDirectoryArgument
  ? path.resolve(referenceDirectoryArgument)
  : path.join(temporaryRoot, "first");
const secondDirectory = path.join(temporaryRoot, "second");

try {
  if (!referenceDirectoryArgument) {
    build(firstDirectory);
  }
  build(secondDirectory);
  execFileSync(
    process.execPath,
    ["scripts/release/compare-artifacts.mjs", firstDirectory, secondDirectory],
    { cwd: repositoryRoot, stdio: "inherit" },
  );
  if (!referenceDirectoryArgument) {
    execFileSync(
      "bash",
      ["scripts/release/smoke-artifacts.sh", firstDirectory, version],
      { cwd: repositoryRoot, stdio: "inherit" },
    );
  }
} finally {
  await rm(temporaryRoot, { force: true, recursive: true });
}

function build(outputDirectory) {
  execFileSync(
    "bash",
    ["scripts/release/build-release.sh", version, outputDirectory],
    { cwd: repositoryRoot, stdio: "inherit" },
  );
}
