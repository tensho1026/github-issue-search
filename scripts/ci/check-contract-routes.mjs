import { readFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";

import { parse } from "yaml";

const repositoryRoot = path.resolve(import.meta.dirname, "../..");
const routerSource = await readFile(
  path.join(repositoryRoot, "apps/api/internal/router/router.go"),
  "utf8",
);
const contract = parse(
  await readFile(
    path.join(repositoryRoot, "packages/contracts/openapi.yaml"),
    "utf8",
  ),
);

const registered = new Set();
const routePattern = /api\.(GET|POST|PUT|PATCH|DELETE)\(\s*"([^"]+)"/g;
for (const match of routerSource.matchAll(routePattern)) {
  const method = match[1].toLowerCase();
  const route = `/api${match[2]}`.replace(/:([A-Za-z][A-Za-z0-9_]*)/g, "{$1}");
  registered.add(`${method} ${route}`);
}

const documented = new Set();
for (const [route, pathItem] of Object.entries(contract.paths ?? {})) {
  for (const method of ["get", "post", "put", "patch", "delete"]) {
    if (pathItem[method]) {
      documented.add(`${method} ${route}`);
    }
  }
}

const missingFromContract = difference(registered, documented);
const missingFromRouter = difference(documented, registered);
if (missingFromContract.length > 0 || missingFromRouter.length > 0) {
  console.error("API route contract drift detected.");
  for (const route of missingFromContract) {
    console.error(`- undocumented router operation: ${route}`);
  }
  for (const route of missingFromRouter) {
    console.error(`- OpenAPI operation missing from router: ${route}`);
  }
  process.exit(1);
}

console.log(
  `${registered.size} router operation(s) match the OpenAPI contract.`,
);

function difference(left, right) {
  return [...left].filter((value) => !right.has(value)).sort();
}
