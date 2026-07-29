import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { readFile, readdir } from "node:fs/promises";
import path from "node:path";
import process from "node:process";

const [artifactDirectoryArgument, version] = process.argv.slice(2);
if (
  !artifactDirectoryArgument ||
  !/^v\d+\.\d+\.\d+(?:[+-][0-9A-Za-z.-]+)?$/.test(version ?? "")
) {
  throw new Error(
    "usage: verify-artifacts.mjs <artifact-directory> <semantic-version>",
  );
}

const artifactDirectory = path.resolve(artifactDirectoryArgument);
const expectedArchives = [
  `issuescout-api-${version}-darwin-amd64.tar.gz`,
  `issuescout-api-${version}-darwin-arm64.tar.gz`,
  `issuescout-api-${version}-linux-amd64.tar.gz`,
  `issuescout-api-${version}-linux-arm64.tar.gz`,
  `issuescout-api-${version}-windows-amd64.tar.gz`,
  `issuescout-web-${version}.tar.gz`,
].sort();
const actualArchives = (await readdir(artifactDirectory))
  .filter((filename) => filename.endsWith(".tar.gz"))
  .sort();

if (JSON.stringify(actualArchives) !== JSON.stringify(expectedArchives)) {
  throw new Error(
    `release archive set differs from contract:\nexpected=${expectedArchives.join(",")}\nactual=${actualArchives.join(",")}`,
  );
}

const checksumSource = await readFile(
  path.join(artifactDirectory, "SHA256SUMS"),
  "utf8",
);
const checksumLines = checksumSource.trim().split("\n");
const checksums = new Map(
  checksumLines.map((line) => {
    const match = line.match(/^([a-f0-9]{64}) {2}([^/\s]+)$/);
    if (!match) {
      throw new Error(`invalid checksum line: ${line}`);
    }
    return [match[2], match[1]];
  }),
);

if (
  JSON.stringify([...checksums.keys()].sort()) !==
  JSON.stringify(expectedArchives)
) {
  throw new Error("SHA256SUMS does not cover exactly the release archives.");
}

for (const archive of expectedArchives) {
  const archivePath = path.join(artifactDirectory, archive);
  const bytes = await readFile(archivePath);
  const digest = createHash("sha256").update(bytes).digest("hex");
  if (digest !== checksums.get(archive)) {
    throw new Error(`checksum mismatch for ${archive}`);
  }

  const entries = execFileSync("tar", ["-tzf", archivePath], {
    encoding: "utf8",
  })
    .trim()
    .split("\n")
    .filter(Boolean);
  const archiveRoot = archive.replace(/\.tar\.gz$/, "");
  if (
    entries.length < 3 ||
    entries.some(
      (entry) =>
        entry.startsWith("/") ||
        entry.includes("../") ||
        (entry !== archiveRoot && !entry.startsWith(`${archiveRoot}/`)),
    )
  ) {
    throw new Error(`unsafe or malformed paths in ${archive}`);
  }

  const manifestSource = execFileSync(
    "tar",
    ["-xOzf", archivePath, `${archiveRoot}/release-manifest.json`],
    { encoding: "utf8" },
  );
  const manifest = JSON.parse(manifestSource);
  if (
    manifest.schemaVersion !== 1 ||
    manifest.version !== version ||
    !/^[a-f0-9]{40}$/.test(manifest.commit) ||
    !["api", "web"].includes(manifest.kind)
  ) {
    throw new Error(`invalid release manifest in ${archive}`);
  }
}

console.log(
  `Verified ${expectedArchives.length} checksummed release archives for ${version}.`,
);
