import { isIP } from "node:net";
import process from "node:process";

const [environmentName, releaseTag, healthURL] = process.argv.slice(2);

if (!["staging", "production"].includes(environmentName)) {
  throw new Error("Promotion environment must be staging or production.");
}
if (!/^v\d+\.\d+\.\d+(?:[+-][0-9A-Za-z.-]+)?$/.test(releaseTag ?? "")) {
  throw new Error(
    "Release tag must use semantic version syntax prefixed by v.",
  );
}

let parsedHealthURL;
try {
  parsedHealthURL = new URL(healthURL);
} catch {
  throw new Error("Health URL must be a valid absolute URL.");
}

const hostname = parsedHealthURL.hostname.toLowerCase();
if (
  parsedHealthURL.protocol !== "https:" ||
  parsedHealthURL.username ||
  parsedHealthURL.password ||
  parsedHealthURL.hash ||
  hostname === "localhost" ||
  hostname.endsWith(".local") ||
  isIP(hostname) !== 0
) {
  throw new Error(
    "Health URL must use HTTPS with a public DNS hostname and no credentials or fragment.",
  );
}

console.log(`Validated ${releaseTag} for ${environmentName}.`);
