import { execFileSync } from "node:child_process";
import { appendFile } from "node:fs/promises";
import process from "node:process";

const [requestedBase, requestedHead = "HEAD"] = process.argv.slice(2);
const zeroSha = /^0{40}$/;
let base = requestedBase;
if (!base || zeroSha.test(base)) {
  base = execFileSync("git", ["rev-list", "--max-parents=0", requestedHead], {
    encoding: "utf8",
  }).trim();
}

const changedFiles = execFileSync(
  "git",
  ["diff", "--name-only", `${base}...${requestedHead}`],
  { encoding: "utf8" },
)
  .trim()
  .split("\n")
  .filter(Boolean);

const matches = (patterns) =>
  changedFiles.some((file) => patterns.some((pattern) => pattern.test(file)));
const shared = [
  /^package\.json$/,
  /^pnpm-lock\.yaml$/,
  /^pnpm-workspace\.yaml$/,
  /^config\//,
  /^scripts\/ci\//,
  /^\.github\/workflows\//,
];
const outputs = {
  backend: matches([/^apps\/api\//, /^go\.work(?:\.sum)?$/, ...shared]),
  contracts: matches([
    /^packages\/contracts\//,
    /^apps\/(?:api|web)\//,
    /^http\//,
    ...shared,
  ]),
  docs: matches([
    /^docs\//,
    /^http\//,
    /^README\.md$/,
    /^CONTRIBUTING\.md$/,
    /^\.github\/(?:ISSUE_TEMPLATE|PULL_REQUEST_TEMPLATE|CODEOWNERS)/,
    ...shared,
  ]),
  delivery: matches([
    /^apps\/(?:api|web)\//,
    /^scripts\/(?:deploy|release)\//,
    /^\.github\/workflows\/(?:deploy|release|security)\.ya?ml$/,
    /^LICENSE$/,
    ...shared,
  ]),
  e2e: matches([
    /^apps\/(?:api|web)\//,
    /^e2e\//,
    /^scripts\/(?:dev|test)\//,
    ...shared,
  ]),
  frontend: matches([/^apps\/web\//, ...shared]),
};

const output = Object.entries(outputs)
  .map(([name, changed]) => `${name}=${String(changed)}`)
  .join("\n");
console.log(`Changed files:\n${changedFiles.join("\n") || "(none)"}`);
console.log(`Detected scopes:\n${output}`);

if (process.env.GITHUB_OUTPUT) {
  await appendFile(process.env.GITHUB_OUTPUT, `${output}\n`);
}
