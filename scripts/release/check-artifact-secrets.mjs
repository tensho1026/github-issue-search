import { execFileSync } from "node:child_process";
import { mkdir, mkdtemp, readdir, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import process from "node:process";

const [artifactDirectoryArgument] = process.argv.slice(2);
if (!artifactDirectoryArgument) {
  throw new Error("usage: check-artifact-secrets.mjs <artifact-directory>");
}

const artifactDirectory = path.resolve(artifactDirectoryArgument);
const archives = (await readdir(artifactDirectory))
  .filter((filename) => filename.endsWith(".tar.gz"))
  .sort();
if (archives.length === 0) {
  throw new Error("no release archives were found for secret inspection");
}

const secretPatterns = [
  /github_pat_[A-Za-z0-9_]{20,}/,
  /gh[pousr]_[A-Za-z0-9]{20,}/,
  /(?:postgres|postgresql):\/\/[^:\s/]+:[^@\s/]+@/i,
  /-----BEGIN (?:EC |OPENSSH |PGP |RSA )?PRIVATE KEY-----/,
  /AKIA[0-9A-Z]{16}/,
];
const forbiddenNames = [
  /(?:^|\/)\.env(?:\.|$)/,
  /\.map$/,
  /\.(?:key|pem|p12|pfx)$/i,
];
const temporaryDirectory = await mkdtemp(
  path.join(tmpdir(), "issuescout-artifact-scan-"),
);

try {
  for (const archive of archives) {
    const destination = path.join(
      temporaryDirectory,
      archive.replace(/\.tar\.gz$/, ""),
    );
    await mkdir(destination);
    execFileSync(
      "tar",
      ["-xzf", path.join(artifactDirectory, archive), "-C", destination],
      { stdio: "ignore" },
    );

    for (const filename of await walk(destination)) {
      const relativeName = path.relative(destination, filename);
      if (forbiddenNames.some((pattern) => pattern.test(relativeName))) {
        throw new Error(
          `${archive} contains forbidden secret-adjacent file ${relativeName}`,
        );
      }

      const contents = await readFile(filename);
      if (contents.length > 100 * 1024 * 1024) {
        throw new Error(`${archive} contains an unexpectedly large file`);
      }
      const text = contents.toString("latin1");
      if (secretPatterns.some((pattern) => pattern.test(text))) {
        throw new Error(
          `${archive} contains content matching a credential pattern`,
        );
      }
      if (
        archive.includes("-web-") &&
        /\b(?:DATABASE_URL|GITHUB_TOKEN|NEON_DATABASE_URL)\b/.test(text)
      ) {
        throw new Error(
          `${archive} exposes a server-only configuration name in the browser bundle`,
        );
      }
    }
  }
} finally {
  await rm(temporaryDirectory, { force: true, recursive: true });
}

console.log(
  `${archives.length} release archive(s) contain no source maps, environment files, key material, or credential-shaped content.`,
);

async function walk(directory) {
  const files = [];
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const target = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      files.push(...(await walk(target)));
    } else if (entry.isFile()) {
      files.push(target);
    }
  }
  return files;
}
