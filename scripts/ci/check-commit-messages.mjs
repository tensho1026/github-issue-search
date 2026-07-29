import { execFileSync } from "node:child_process";
import process from "node:process";

const [base, head = "HEAD"] = process.argv.slice(2);
if (!base) {
  console.error("Usage: check-commit-messages.mjs <base-sha> [head-sha]");
  process.exit(2);
}

const messages = execFileSync(
  "git",
  ["log", "--format=%s", "--no-merges", `${base}..${head}`],
  { encoding: "utf8" },
)
  .trim()
  .split("\n")
  .filter(Boolean);
const conventionalCommit =
  /^(?:build|chore|ci|docs|feat|fix|perf|refactor|revert|test)(?:\([a-z0-9._/-]+\))?!?: [^\s].{2,71}$/;
const invalid = messages.filter((message) => !conventionalCommit.test(message));

if (messages.length === 0) {
  console.error("The pull request must contain at least one non-merge commit.");
  process.exit(1);
}
if (invalid.length > 0) {
  console.error("Invalid conventional commit subjects:");
  for (const message of invalid) {
    console.error(`- ${message}`);
  }
  process.exit(1);
}

console.log(`${messages.length} commit subject(s) passed policy checks.`);
