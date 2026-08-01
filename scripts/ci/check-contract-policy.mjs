import { readFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";

import { parse } from "yaml";

const repositoryRoot = path.resolve(import.meta.dirname, "../..");
const contract = parse(
  await readFile(
    path.join(repositoryRoot, "packages/contracts/openapi.yaml"),
    "utf8",
  ),
);
const methods = ["get", "post", "put", "patch", "delete"];
const requiredOperationalStatuses = ["403", "500", "504"];
const failures = [];
const operationIDs = new Set();

for (const [route, pathItem] of Object.entries(contract.paths ?? {})) {
  for (const method of methods) {
    const operation = pathItem[method];
    if (!operation) {
      continue;
    }
    const location = `${method.toUpperCase()} ${route}`;
    if (!operation.operationId || operationIDs.has(operation.operationId)) {
      failures.push(`${location}: operationId must be present and unique`);
    }
    operationIDs.add(operation.operationId);

    const responses = operation.responses ?? {};
    if (Object.hasOwn(responses, "default")) {
      failures.push(`${location}: default responses are forbidden`);
    }
    for (const status of requiredOperationalStatuses) {
      if (!responses[status]) {
        failures.push(`${location}: status ${status} is undocumented`);
      }
    }

    const successStatuses = Object.keys(responses).filter((status) =>
      /^[23]\d\d$/.test(status),
    );
    if (successStatuses.length !== 1) {
      failures.push(
        `${location}: exactly one explicit success status is required`,
      );
    }

    for (const [status, unresolvedResponse] of Object.entries(responses)) {
      if (!/^[1-5]\d\d$/.test(status)) {
        failures.push(`${location}: response status ${status} is not explicit`);
        continue;
      }
      const response = resolveLocalReference(unresolvedResponse);
      const requestID = resolveLocalReference(
        response?.headers?.["X-Request-ID"],
      );
      if (
        !requestID ||
        requestID.schema?.type !== "string" ||
        requestID.required !== true
      ) {
        failures.push(
          `${location} ${status}: required X-Request-ID header is undocumented`,
        );
      }

      const schema =
        response?.content?.["application/json"]?.schema?.$ref ?? "";
      if (/^[45]\d\d$/.test(status)) {
        if (schema !== "#/components/schemas/ErrorEnvelope") {
          failures.push(`${location} ${status}: errors must use ErrorEnvelope`);
        }
      } else if (/^3\d\d$/.test(status)) {
        const redirectLocation = resolveLocalReference(
          response?.headers?.Location,
        );
        if (
          !redirectLocation ||
          redirectLocation.required !== true ||
          redirectLocation.schema?.format !== "uri"
        ) {
          failures.push(
            `${location} ${status}: redirect must declare a required URI Location header`,
          );
        }
        if (response?.content) {
          failures.push(
            `${location} ${status}: redirect must not declare a response body`,
          );
        }
      } else if (!schema.endsWith("Envelope")) {
        failures.push(`${location} ${status}: success must use an envelope`);
      }
    }
  }
}

if (failures.length > 0) {
  console.error("OpenAPI policy violations:");
  for (const failure of failures) {
    console.error(`- ${failure}`);
  }
  process.exit(1);
}

console.log(
  `${operationIDs.size} OpenAPI operation(s) declare strict statuses, envelopes or redirects, and request correlation.`,
);

function resolveLocalReference(value) {
  if (!value?.$ref) {
    return value;
  }
  if (!value.$ref.startsWith("#/")) {
    throw new Error(`remote OpenAPI references are forbidden: ${value.$ref}`);
  }
  return value.$ref
    .slice(2)
    .split("/")
    .reduce((current, segment) => current?.[segment], contract);
}
