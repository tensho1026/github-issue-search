import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";

import { parse } from "yaml";

const repositoryRoot = path.resolve(import.meta.dirname, "../..");
const workflowDirectory = path.join(repositoryRoot, ".github/workflows");
const workflowFiles = (await readdir(workflowDirectory))
  .filter((file) => /\.ya?ml$/.test(file))
  .sort();
const violations = [];

if (workflowFiles.length === 0) {
  violations.push("No GitHub Actions workflows were found.");
}

for (const filename of workflowFiles) {
  const relativePath = `.github/workflows/${filename}`;
  const source = await readFile(path.join(workflowDirectory, filename), "utf8");
  const workflow = parse(source);

  const events = workflow.on;
  if (
    events === "pull_request_target" ||
    (events &&
      typeof events === "object" &&
      Object.hasOwn(events, "pull_request_target"))
  ) {
    violations.push(`${relativePath}: pull_request_target is forbidden`);
  }
  if (!workflow.permissions || typeof workflow.permissions !== "object") {
    violations.push(`${relativePath}: top-level permissions must be explicit`);
  } else {
    for (const [scope, access] of Object.entries(workflow.permissions)) {
      if (access === "write" || access === "write-all") {
        violations.push(
          `${relativePath}: top-level ${scope} permission may not be write`,
        );
      }
    }
  }
  if (!workflow.concurrency) {
    violations.push(`${relativePath}: concurrency cancellation is required`);
  }

  for (const [jobName, job] of Object.entries(workflow.jobs ?? {})) {
    if (
      typeof job["timeout-minutes"] !== "number" ||
      job["timeout-minutes"] <= 0
    ) {
      violations.push(
        `${relativePath}: job ${jobName} needs a positive timeout-minutes`,
      );
    }
    for (const step of job.steps ?? []) {
      if (!step.uses || step.uses.startsWith("./")) {
        continue;
      }
      if (!/@[a-f0-9]{40}$/.test(step.uses)) {
        violations.push(
          `${relativePath}: action ${step.uses} is not pinned to a commit SHA`,
        );
      }
    }
  }

  if (/\bsecrets:\s*inherit\b/.test(source)) {
    violations.push(`${relativePath}: secrets inheritance is forbidden`);
  }
}

if (violations.length > 0) {
  console.error("Workflow policy violations:");
  for (const violation of violations) {
    console.error(`- ${violation}`);
  }
  process.exit(1);
}

console.log(`${workflowFiles.length} workflow file(s) passed policy checks.`);
