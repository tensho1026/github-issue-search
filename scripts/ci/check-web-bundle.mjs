import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";
import { gzipSync } from "node:zlib";

import budgets from "../../config/quality-budgets.json" with { type: "json" };

const repositoryRoot = path.resolve(import.meta.dirname, "../..");
const assetsDirectory = path.join(repositoryRoot, "apps/web/dist/assets");
const assets = (await walk(assetsDirectory)).filter((file) =>
  /\.(?:css|js)$/.test(file),
);

if (assets.length === 0) {
  console.error("No production web assets found. Run the web build first.");
  process.exit(1);
}

let totalGzipBytes = 0;
let largestJavaScript = { bytes: 0, file: "" };
for (const asset of assets) {
  const gzipBytes = gzipSync(await readFile(asset)).byteLength;
  totalGzipBytes += gzipBytes;
  if (asset.endsWith(".js") && gzipBytes > largestJavaScript.bytes) {
    largestJavaScript = { bytes: gzipBytes, file: asset };
  }
}

const totalGzipKiB = toKiB(totalGzipBytes);
const largestJavaScriptKiB = toKiB(largestJavaScript.bytes);
const failures = [];
if (totalGzipKiB > budgets.webBundle.maximumTotalGzipKiB) {
  failures.push(
    `total gzip size ${totalGzipKiB} KiB exceeds ${budgets.webBundle.maximumTotalGzipKiB} KiB`,
  );
}
if (largestJavaScriptKiB > budgets.webBundle.maximumSingleJavaScriptGzipKiB) {
  failures.push(
    `largest JavaScript asset ${largestJavaScriptKiB} KiB exceeds ${budgets.webBundle.maximumSingleJavaScriptGzipKiB} KiB`,
  );
}

console.log(
  `Web bundle: total=${totalGzipKiB} KiB, largest-js=${largestJavaScriptKiB} KiB (${path.basename(largestJavaScript.file)})`,
);
if (failures.length > 0) {
  for (const failure of failures) {
    console.error(`- ${failure}`);
  }
  process.exit(1);
}

function toKiB(bytes) {
  return Math.round((bytes / 1024) * 100) / 100;
}

async function walk(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const files = [];
  for (const entry of entries) {
    const target = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      files.push(...(await walk(target)));
    } else {
      files.push(target);
    }
  }
  return files;
}
