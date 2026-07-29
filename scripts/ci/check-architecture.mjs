import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";

const repositoryRoot = path.resolve(import.meta.dirname, "../..");
const apiRoot = path.join(repositoryRoot, "apps/api");

const rules = [
  {
    directory: "internal/domain",
    forbidden: [
      "internal/cache",
      "internal/client",
      "internal/config",
      "internal/handler",
      "internal/middleware",
      "internal/router",
      "internal/transport",
      "internal/usecase",
    ],
  },
  {
    directory: "internal/port",
    forbidden: [
      "internal/cache",
      "internal/client",
      "internal/config",
      "internal/handler",
      "internal/middleware",
      "internal/router",
      "internal/transport",
      "internal/usecase",
    ],
  },
  {
    directory: "internal/usecase",
    forbidden: [
      "internal/cache",
      "internal/client",
      "internal/config",
      "internal/handler",
      "internal/middleware",
      "internal/router",
      "internal/transport",
    ],
  },
  {
    directory: "internal/handler",
    forbidden: [
      "internal/cache",
      "internal/client",
      "internal/config",
      "internal/middleware",
      "internal/router",
    ],
  },
  {
    directory: "internal/client",
    forbidden: [
      "internal/cache",
      "internal/config",
      "internal/handler",
      "internal/middleware",
      "internal/router",
      "internal/transport",
      "internal/usecase",
    ],
  },
  {
    directory: "internal/cache",
    forbidden: [
      "internal/client",
      "internal/config",
      "internal/handler",
      "internal/middleware",
      "internal/router",
      "internal/transport",
      "internal/usecase",
    ],
  },
];

const violations = [];
for (const rule of rules) {
  const absoluteDirectory = path.join(apiRoot, rule.directory);
  for (const file of await walk(absoluteDirectory)) {
    if (!file.endsWith(".go") || file.endsWith("_test.go")) {
      continue;
    }
    const source = await readFile(file, "utf8");
    for (const forbidden of rule.forbidden) {
      const forbiddenImport =
        "github.com/tensho1026/github-issue-search/apps/api/" + forbidden;
      if (source.includes(forbiddenImport)) {
        violations.push(
          `${path.relative(repositoryRoot, file)} imports forbidden layer ${forbidden}`,
        );
      }
    }
  }
}

if (violations.length > 0) {
  console.error("Architecture boundary violations:");
  for (const violation of violations) {
    console.error(`- ${violation}`);
  }
  process.exit(1);
}

console.log("Architecture dependency directions are valid.");

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
