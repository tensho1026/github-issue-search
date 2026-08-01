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
const operationsRequiringExamples = new Set([
  "analyzeGitHubProfile",
  "deleteAccount",
  "exportAccountData",
  "getAccountPreferences",
  "getAuthSession",
  "getDatabaseHealth",
  "getGitHubUser",
  "getHealth",
  "getIssueRecommendation",
  "listAccountBookmarks",
  "listAccountSavedSearches",
  "logoutAuthSession",
  "searchGitHubIssues",
  "searchGitHubRepositories",
]);
const cacheAwareOperations = new Set([
  "getIssueRecommendation",
  "searchGitHubIssues",
  "searchGitHubRepositories",
]);
const failures = [];
const operationIDs = new Set();

for (const [route, pathItem] of Object.entries(contract.paths ?? {})) {
  for (const method of methods) {
    const operation = pathItem[method];
    if (!operation) {
      continue;
    }
    const location = `${method.toUpperCase()} ${route}`;
    if (!operation.summary || !operation.description) {
      failures.push(
        `${location}: summary and detailed description are required`,
      );
    }
    if (!Array.isArray(operation.tags) || operation.tags.length === 0) {
      failures.push(`${location}: at least one operation tag is required`);
    }
    if (!operation.operationId || operationIDs.has(operation.operationId)) {
      failures.push(`${location}: operationId must be present and unique`);
    }
    operationIDs.add(operation.operationId);

    const parameters = [
      ...(pathItem.parameters ?? []),
      ...(operation.parameters ?? []),
    ].map(resolveLocalReference);
    if (
      !parameters.some(
        (parameter) =>
          parameter?.in === "header" &&
          parameter?.name?.toLowerCase() === "x-request-id",
      )
    ) {
      failures.push(`${location}: optional X-Request-ID input is undocumented`);
    }
    for (const parameter of parameters) {
      if (!parameter?.description) {
        failures.push(
          `${location}: ${parameter?.in ?? "unknown"} parameter ${parameter?.name ?? "unknown"} has no description`,
        );
      }
    }

    if (operation.requestBody) {
      const requestBody = resolveLocalReference(operation.requestBody);
      if (!requestBody.description) {
        failures.push(`${location}: request body has no description`);
      }
      if (!requestBody.content?.["application/json"]?.schema) {
        failures.push(
          `${location}: request body must declare an application/json schema`,
        );
      }
    }

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
      if (!response?.description) {
        failures.push(`${location} ${status}: response has no description`);
      }
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

    if (operationsRequiringExamples.has(operation.operationId)) {
      const hasExample = successStatuses.some((status) => {
        const response = resolveLocalReference(responses[status]);
        const mediaType = response?.content?.["application/json"];
        return (
          mediaType?.example !== undefined ||
          Object.keys(mediaType?.examples ?? {}).length > 0
        );
      });
      if (!hasExample) {
        failures.push(
          `${location}: common success flow needs a realistic example`,
        );
      }
    }

    if (cacheAwareOperations.has(operation.operationId)) {
      const response = resolveLocalReference(responses[successStatuses[0]]);
      const cacheHeader = response?.headers?.["X-IssueScout-Cache"]?.$ref ?? "";
      if (cacheHeader !== "#/components/headers/CacheStatus") {
        failures.push(
          `${location}: cache behavior must use the shared CacheStatus header`,
        );
      }
    }
  }
}

for (const responseName of ["InvalidRequest", "GitHubRateLimited"]) {
  const examples =
    contract.components?.responses?.[responseName]?.content?.[
      "application/json"
    ]?.examples ?? {};
  if (Object.keys(examples).length === 0) {
    failures.push(
      `components.responses.${responseName}: a realistic error example is required`,
    );
  }
}

walkContract(contract);

if (failures.length > 0) {
  console.error("OpenAPI policy violations:");
  for (const failure of failures) {
    console.error(`- ${failure}`);
  }
  process.exit(1);
}

console.log(
  `${operationIDs.size} OpenAPI operation(s) declare detailed inputs, strict statuses, examples, cache semantics, envelopes or redirects, and request correlation.`,
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

function walkContract(value) {
  if (Array.isArray(value)) {
    for (const item of value) {
      walkContract(item);
    }
    return;
  }
  if (!value || typeof value !== "object") {
    return;
  }
  for (const [key, item] of Object.entries(value)) {
    if (key === "$ref" && !item.startsWith("#/")) {
      failures.push(`remote OpenAPI reference is forbidden: ${item}`);
    }
    if (key === "externalValue") {
      failures.push(`external example value is forbidden: ${item}`);
    }
    walkContract(item);
  }
}
