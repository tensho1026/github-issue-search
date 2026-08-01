import { access, readFile, readdir } from "node:fs/promises";
import path from "node:path";
import process from "node:process";

import { JSDOM } from "jsdom";
import { parse } from "yaml";

const dom = new JSDOM("<!doctype html><html><body></body></html>");
globalThis.window = dom.window;
globalThis.document = dom.window.document;
globalThis.Element = dom.window.Element;
globalThis.SVGElement = dom.window.SVGElement;
const mermaid = (await import("mermaid")).default;

const repositoryRoot = path.resolve(import.meta.dirname, "../..");
const docsRoot = path.join(repositoryRoot, "docs");
const packageJSON = JSON.parse(
  await readFile(path.join(repositoryRoot, "package.json"), "utf8"),
);
const makefile = await readFile(path.join(repositoryRoot, "Makefile"), "utf8");
const contract = parse(
  await readFile(
    path.join(repositoryRoot, "packages/contracts/openapi.yaml"),
    "utf8",
  ),
);
const markdownFiles = [
  path.join(repositoryRoot, "README.md"),
  path.join(repositoryRoot, "CONTRIBUTING.md"),
  ...(await walk(docsRoot)).filter((filename) => filename.endsWith(".md")),
];
const sources = await Promise.all(
  markdownFiles.map(async (filename) => ({
    filename,
    source: await readFile(filename, "utf8"),
  })),
);
const documentation = sources.map(({ source }) => source).join("\n");
const failures = [];
let mermaidDiagramCount = 0;

for (const { filename, source } of sources) {
  for (const match of source.matchAll(
    /!?\[[^\]]*]\(([^)\s]+)(?:\s+[^)]*)?\)/g,
  )) {
    const rawTarget = match[1].replace(/^<|>$/g, "");
    if (rawTarget.startsWith("#") || /^(?:https?:|mailto:)/.test(rawTarget)) {
      continue;
    }
    const localTarget = decodeURIComponent(rawTarget.split(/[?#]/, 1)[0]);
    const absoluteTarget = path.resolve(path.dirname(filename), localTarget);
    try {
      await access(absoluteTarget);
    } catch {
      failures.push(
        `${path.relative(repositoryRoot, filename)}: broken link ${rawTarget}`,
      );
    }
  }

  let diagramIndex = 0;
  for (const match of source.matchAll(/```mermaid\s*\n([\s\S]*?)```/gu)) {
    diagramIndex++;
    mermaidDiagramCount++;
    try {
      await mermaid.parse(match[1]);
    } catch (error) {
      failures.push(
        `${path.relative(repositoryRoot, filename)}: Mermaid diagram ${diagramIndex} is invalid: ${error instanceof Error ? error.message : String(error)}`,
      );
    }
  }
}

const rootScripts = Object.keys(packageJSON.scripts ?? {}).sort();
for (const script of rootScripts) {
  if (!documentation.includes(`pnpm run ${script}`)) {
    failures.push(`root command is undocumented: pnpm run ${script}`);
  }
}

const makeTargets = [...makefile.matchAll(/^([a-z][a-z0-9_-]+):.*## /gm)]
  .map((match) => match[1])
  .sort();
for (const target of makeTargets) {
  if (!documentation.includes(`make ${target}`)) {
    failures.push(`Make target is undocumented: make ${target}`);
  }
}

const configurationSources = await Promise.all([
  readFile(
    path.join(repositoryRoot, "apps/api/internal/config/config.go"),
    "utf8",
  ),
  readFile(path.join(repositoryRoot, "scripts/dev/native-stack.mjs"), "utf8"),
]);
const environmentVariables = new Set();
for (const match of configurationSources[0].matchAll(/"([A-Z][A-Z0-9_]+)"/g)) {
  environmentVariables.add(match[1]);
}
for (const match of configurationSources[1].matchAll(
  /process\.env\.([A-Z][A-Z0-9_]*)/g,
)) {
  environmentVariables.add(match[1]);
}
for (const variable of [...environmentVariables].sort()) {
  if (!documentation.includes(`\`${variable}\``)) {
    failures.push(`environment variable is undocumented: ${variable}`);
  }
}

const apiGuide = await readFile(path.join(docsRoot, "api.md"), "utf8");
for (const route of Object.keys(contract.paths ?? {})) {
  if (!apiGuide.includes(`\`${route}\``)) {
    failures.push(`API path is missing from docs/api.md: ${route}`);
  }
}
for (const code of contract.components?.schemas?.ErrorEnvelope?.properties
  ?.error?.properties?.code?.enum ?? []) {
  if (!apiGuide.includes(`\`${code}\``)) {
    failures.push(`API error code is missing from docs/api.md: ${code}`);
  }
}

for (const { filename, source } of sources) {
  for (const match of source.matchAll(/\bpnpm run ([a-z][a-z0-9:-]*)/g)) {
    if (!packageJSON.scripts?.[match[1]]) {
      failures.push(
        `${path.relative(repositoryRoot, filename)}: unknown command pnpm run ${match[1]}`,
      );
    }
  }
  for (const match of source.matchAll(/(?:^|\n|`)make ([a-z][a-z0-9_-]*)/g)) {
    if (!makeTargets.includes(match[1])) {
      failures.push(
        `${path.relative(repositoryRoot, filename)}: unknown Make target make ${match[1]}`,
      );
    }
  }
}

if (failures.length > 0) {
  console.error("Documentation contract violations:");
  for (const failure of failures) {
    console.error(`- ${failure}`);
  }
  process.exit(1);
}

console.log(
  `${markdownFiles.length} Markdown file(s), ${rootScripts.length} root command(s), ${makeTargets.length} Make target(s), ${environmentVariables.size} environment variable(s), and all API paths/error codes are documented.`,
);
console.log(`${mermaidDiagramCount} Mermaid diagram(s) parsed successfully.`);

async function walk(directory) {
  const files = [];
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const target = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      files.push(...(await walk(target)));
    } else {
      files.push(target);
    }
  }
  return files;
}
