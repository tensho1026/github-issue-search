import { readFile, readdir } from "node:fs/promises";
import path from "node:path";
import process from "node:process";

import { parse } from "yaml";

const repositoryRoot = path.resolve(import.meta.dirname, "../..");
const forbiddenBasenames = [
  /^\.dockerignore$/i,
  /^(?:docker|container)file(?:\..+)?$/i,
  /^(?:docker-)?compose(?:\.[^.]+)?\.ya?ml$/i,
];
const runtimeCommand =
  /(?:^|[\s;&|])(docker|podman|nerdctl|buildah|kaniko)(?:[\s;&|]|$)/i;
const dependencyName =
  /(?:^|[/_.-])(docker|containerd|podman|buildah|kaniko|oci)(?:$|[/_.-])/i;
const failures = [];

for (const filename of await walk(repositoryRoot)) {
  const relativeName = path.relative(repositoryRoot, filename);
  if (
    forbiddenBasenames.some((pattern) =>
      pattern.test(path.basename(relativeName)),
    )
  ) {
    failures.push(`${relativeName}: container configuration file is forbidden`);
  }
}

const packageFiles = (await walk(repositoryRoot)).filter((filename) =>
  filename.endsWith("package.json"),
);
for (const filename of packageFiles) {
  const manifest = JSON.parse(await readFile(filename, "utf8"));
  for (const section of [
    "dependencies",
    "devDependencies",
    "optionalDependencies",
    "peerDependencies",
  ]) {
    for (const dependency of Object.keys(manifest[section] ?? {})) {
      if (dependencyName.test(dependency)) {
        failures.push(
          `${path.relative(repositoryRoot, filename)}: ${section} contains forbidden ${dependency}`,
        );
      }
    }
  }
  for (const [name, command] of Object.entries(manifest.scripts ?? {})) {
    if (runtimeCommand.test(command)) {
      failures.push(
        `${path.relative(repositoryRoot, filename)}: script ${name} invokes a container runtime`,
      );
    }
  }
}

const goModule = await readFile(
  path.join(repositoryRoot, "apps/api/go.mod"),
  "utf8",
);
for (const line of goModule.split("\n")) {
  const moduleName = line.trim().split(/\s+/, 1)[0];
  if (moduleName && dependencyName.test(moduleName)) {
    failures.push(
      `apps/api/go.mod: forbidden container dependency ${moduleName}`,
    );
  }
}

const workflowDirectory = path.join(repositoryRoot, ".github/workflows");
for (const filename of await readdir(workflowDirectory)) {
  if (!/\.ya?ml$/.test(filename)) {
    continue;
  }
  const relativeName = `.github/workflows/${filename}`;
  const workflow = parse(
    await readFile(path.join(workflowDirectory, filename), "utf8"),
  );
  for (const [jobName, job] of Object.entries(workflow.jobs ?? {})) {
    if (job.container || job.services) {
      failures.push(
        `${relativeName}: job ${jobName} uses a container or service`,
      );
    }
    for (const step of job.steps ?? []) {
      if (
        typeof step.uses === "string" &&
        (/^docker:\/\//i.test(step.uses) ||
          /(?:^|\/)(?:docker|build-push|setup-buildx)(?:\/|@)/i.test(step.uses))
      ) {
        failures.push(
          `${relativeName}: job ${jobName} uses container action ${step.uses}`,
        );
      }
      if (typeof step.run === "string" && runtimeCommand.test(step.run)) {
        failures.push(
          `${relativeName}: job ${jobName} invokes a container runtime`,
        );
      }
    }
  }
}

if (failures.length > 0) {
  console.error("Docker-free repository policy violations:");
  for (const failure of failures) {
    console.error(`- ${failure}`);
  }
  process.exit(1);
}

console.log(
  "Repository contains no Docker/OCI configuration, runtime commands, workflow containers, or container dependencies.",
);

async function walk(directory) {
  const files = [];
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    if (
      entry.isDirectory() &&
      [".git", "artifacts", "coverage", "dist", "node_modules"].includes(
        entry.name,
      )
    ) {
      continue;
    }
    const target = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      files.push(...(await walk(target)));
    } else if (entry.isFile()) {
      files.push(target);
    }
  }
  return files;
}
