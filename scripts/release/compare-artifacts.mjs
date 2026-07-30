import { createHash } from "node:crypto";
import { createReadStream } from "node:fs";
import { readdir } from "node:fs/promises";
import path from "node:path";
import process from "node:process";

const [firstDirectoryArgument, secondDirectoryArgument] = process.argv.slice(2);
if (!firstDirectoryArgument || !secondDirectoryArgument) {
  throw new Error(
    "usage: compare-artifacts.mjs <first-directory> <second-directory>",
  );
}

const firstDirectory = path.resolve(firstDirectoryArgument);
const secondDirectory = path.resolve(secondDirectoryArgument);
const firstFiles = await releaseFiles(firstDirectory);
const secondFiles = await releaseFiles(secondDirectory);

if (JSON.stringify(firstFiles) !== JSON.stringify(secondFiles)) {
  throw new Error(
    `independent builds produced different file sets:\nfirst=${firstFiles.join(",")}\nsecond=${secondFiles.join(",")}`,
  );
}

for (const filename of firstFiles) {
  const [firstDigest, secondDigest] = await Promise.all([
    digest(path.join(firstDirectory, filename)),
    digest(path.join(secondDirectory, filename)),
  ]);
  if (firstDigest !== secondDigest) {
    throw new Error(
      `independent builds are not reproducible: ${filename} has different bytes`,
    );
  }
}

console.log(
  `${firstFiles.length} release file(s) are byte-identical across independent builds.`,
);

async function releaseFiles(directory) {
  return (await readdir(directory, { withFileTypes: true }))
    .filter(
      (entry) =>
        entry.isFile() &&
        (entry.name.endsWith(".tar.gz") || entry.name === "SHA256SUMS"),
    )
    .map((entry) => entry.name)
    .sort();
}

function digest(filename) {
  return new Promise((resolve, reject) => {
    const hash = createHash("sha256");
    createReadStream(filename)
      .on("data", (chunk) => hash.update(chunk))
      .on("error", reject)
      .on("end", () => resolve(hash.digest("hex")));
  });
}
