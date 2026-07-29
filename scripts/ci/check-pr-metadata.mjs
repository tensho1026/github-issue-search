import { readFile } from "node:fs/promises";
import process from "node:process";

if (!process.env.GITHUB_EVENT_PATH) {
  console.error("GITHUB_EVENT_PATH is required.");
  process.exit(2);
}

const event = JSON.parse(await readFile(process.env.GITHUB_EVENT_PATH, "utf8"));
const pullRequest = event.pull_request;
if (!pullRequest) {
  console.log("Not a pull request event; metadata policy is not applicable.");
  process.exit(0);
}

const body = pullRequest.body ?? "";
const violations = [];
if (!/\b(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?)\s+#\d+\b/i.test(body)) {
  violations.push("include a closing keyword such as `Closes #13`");
}

for (const heading of [
  "Summary",
  "Related issue",
  "Validation",
  "Performance impact",
  "Security impact",
  "Self-review",
]) {
  const expression = new RegExp(`^##\\s+${escapeRegExp(heading)}\\s*$`, "im");
  if (!expression.test(body)) {
    violations.push(`include the \`## ${heading}\` section`);
  }
}

if (!pullRequest.draft && /^\s*-\s+\[\s\]/m.test(body)) {
  violations.push(
    "complete every self-review checkbox before marking the PR ready",
  );
}

if (violations.length > 0) {
  console.error("Pull request metadata policy failed:");
  for (const violation of violations) {
    console.error(`- ${violation}`);
  }
  process.exit(1);
}

console.log("Pull request metadata passed policy checks.");

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}
