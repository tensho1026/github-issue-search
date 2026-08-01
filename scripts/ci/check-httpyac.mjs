import { spawn } from "node:child_process";
import { readFile, readdir } from "node:fs/promises";
import http from "node:http";
import path from "node:path";
import process from "node:process";

import { parse } from "yaml";

const repositoryRoot = path.resolve(import.meta.dirname, "../..");
const httpRoot = path.join(repositoryRoot, "http");
const requestMethods = new Set([
  "DELETE",
  "GET",
  "HEAD",
  "PATCH",
  "POST",
  "PUT",
]);
const operationalPaths = new Set([
  "GET /docs/",
  "GET /docs/not-an-asset.js",
  "GET /openapi.yaml",
]);
const negativeRequired = new Set([
  "GET /api/auth/session",
  "GET /api/github/users/{username}",
  "GET /api/issues/{owner}/{repository}/{issueNumber}",
  "POST /api/issues/search",
  "POST /api/repositories/search",
  "PUT /api/account/preferences",
]);
const anonymousOperations = new Set([
  "GET /api/github/users/{username}",
  "GET /api/github/users/{username}/profile-analysis",
  "GET /api/health",
  "GET /api/health/database",
  "GET /api/issues/{owner}/{repository}/{issueNumber}",
  "POST /api/issues/search",
  "POST /api/repositories/search",
]);
const privateEnvironmentVariables = [
  "csrfToken",
  "oauthCode",
  "oauthFlowCookie",
  "oauthState",
  "sessionToken",
];

const contract = parse(
  await readFile(
    path.join(repositoryRoot, "packages/contracts/openapi.yaml"),
    "utf8",
  ),
);
const operations = [];
for (const [contractPath, pathItem] of Object.entries(contract.paths ?? {})) {
  for (const [method, operation] of Object.entries(pathItem)) {
    const upperMethod = method.toUpperCase();
    if (!requestMethods.has(upperMethod) || !operation?.operationId) {
      continue;
    }
    operations.push({
      key: `${upperMethod} ${contractPath}`,
      method: upperMethod,
      operationId: operation.operationId,
      path: contractPath,
    });
  }
}

const filenames = (await readdir(httpRoot))
  .filter((filename) => filename.endsWith(".http"))
  .sort();
const requests = [];
const names = new Set();
const failures = [];
for (const filename of filenames) {
  const source = await readFile(path.join(httpRoot, filename), "utf8");
  if (
    /postgres(?:ql)?:\/\//iu.test(source) ||
    /\b(?:ghp_|github_pat_)[A-Za-z0-9_]+/u.test(source) ||
    /\bAuthorization:\s*Bearer\s+\S+/iu.test(source)
  ) {
    failures.push(
      `${filename}: request collection contains a credential shape`,
    );
  }

  const regions = source
    .split(/(?=^###\s)/mu)
    .filter((region) => region.startsWith("###"));
  for (const region of regions) {
    const titleMatch = /^###\s+(.+)$/mu.exec(region);
    const nameMatch = /^#\s*@name\s+([A-Za-z][A-Za-z0-9]*)\s*$/mu.exec(region);
    const requestMatches = [
      ...region.matchAll(/^(DELETE|GET|HEAD|PATCH|POST|PUT)\s+(\S+)\s*$/gmu),
    ];
    const title = titleMatch?.[1].trim() ?? "";
    const name = nameMatch?.[1] ?? "";
    const lineNumber =
      source.slice(0, source.indexOf(region)).split(/\r?\n/u).length +
      (requestMatches[0]?.index === undefined
        ? 0
        : region.slice(0, requestMatches[0].index).split(/\r?\n/u).length - 1);

    if (requestMatches.length !== 1) {
      failures.push(
        `${filename}:${lineNumber}: each ### region needs exactly one request`,
      );
      continue;
    }
    const [, method, rawURL] = requestMatches[0];
    if (!title) {
      failures.push(`${filename}:${lineNumber}: request needs a ### title`);
    }
    if (!name) {
      failures.push(
        `${filename}:${lineNumber}: request needs a unique # @name`,
      );
    } else if (names.has(name)) {
      failures.push(
        `${filename}:${lineNumber}: duplicate request name ${name}`,
      );
    } else {
      names.add(name);
    }
    if (!rawURL.startsWith("{{apiBaseUrl}}/")) {
      failures.push(
        `${filename}:${lineNumber}: URL must use the shared apiBaseUrl`,
      );
    }

    const requestPath = normalizeRequestPath(rawURL);
    const matchingOperation = operations.find(
      (operation) =>
        operation.method === method && pathsMatch(operation.path, requestPath),
    );
    const request = {
      contractKey: matchingOperation?.key,
      filename,
      method,
      name,
      negative: title.startsWith("[negative]"),
      path: requestPath,
      source: region,
    };
    requests.push(request);

    if (
      !matchingOperation &&
      !operationalPaths.has(`${method} ${requestPath}`)
    ) {
      failures.push(
        `${filename}:${lineNumber}: ${method} ${requestPath} is not in OpenAPI`,
      );
    }
    if (
      matchingOperation &&
      anonymousOperations.has(matchingOperation.key) &&
      /^(?:Cookie|Authorization|X-CSRF-Token):/imu.test(request.source)
    ) {
      failures.push(
        `${filename}:${lineNumber}: anonymous request contains account credentials`,
      );
    }
    for (const cookieLine of request.source.matchAll(/^Cookie:\s*(.+)$/gimu)) {
      const cookieValues = cookieLine[1].split(";");
      if (
        cookieValues.some(
          (cookie) =>
            !/^\s*[A-Za-z0-9_-]+=\{\{[A-Za-z][A-Za-z0-9]*\}\}\s*$/u.test(
              cookie,
            ),
        )
      ) {
        failures.push(
          `${filename}:${lineNumber}: every Cookie value must be an environment placeholder`,
        );
      }
    }
  }
}

const coveredOperations = new Set(
  requests.map((request) => request.contractKey).filter(Boolean),
);
for (const operation of operations) {
  if (!coveredOperations.has(operation.key)) {
    failures.push(
      `${operation.key} (${operation.operationId}) has no HTTPYAC request`,
    );
  }
}
for (const operation of negativeRequired) {
  if (
    !requests.some(
      (request) => request.contractKey === operation && request.negative,
    )
  ) {
    failures.push(`${operation} needs a [negative] HTTPYAC request`);
  }
}

const environmentSource = await readFile(
  path.join(httpRoot, "http-client.env.json"),
  "utf8",
);
if (
  /postgres(?:ql)?:\/\//iu.test(environmentSource) ||
  /\b(?:ghp_|github_pat_)[A-Za-z0-9_]+/u.test(environmentSource)
) {
  failures.push("http-client.env.json contains a credential shape");
}
const environmentFile = JSON.parse(environmentSource);
for (const environment of ["ci", "deployed", "local"]) {
  if (!environmentFile[environment]?.apiBaseUrl) {
    failures.push(`http-client.env.json is missing ${environment}.apiBaseUrl`);
  }
  const privatePrefix =
    environment === "ci"
      ? "ci-placeholder-"
      : environment === "deployed"
        ? "inject-from-"
        : "replace-with-";
  for (const variable of privateEnvironmentVariables) {
    const value = environmentFile[environment]?.[variable];
    if (typeof value !== "string" || !value.startsWith(privatePrefix)) {
      failures.push(
        `http-client.env.json ${environment}.${variable} must remain an inert ${privatePrefix} placeholder`,
      );
    }
  }
  if (environmentFile[environment]?.accountDeleteConfirmation === "DELETE") {
    failures.push(
      `http-client.env.json ${environment}.accountDeleteConfirmation must remain destructive-safe`,
    );
  }
}

if (failures.length > 0) {
  reportFailures(failures);
}

let receivedRequests = 0;
const server = http.createServer((request, response) => {
  receivedRequests++;
  request.resume();
  response.writeHead(200, {
    "Content-Type": "application/json",
    "Set-Cookie": "issuescout_ci_placeholder=1; Path=/; HttpOnly",
  });
  response.end('{"data":{"ok":true},"meta":{"requestId":"httpyac-ci"}}');
});
await new Promise((resolve, reject) => {
  server.once("error", reject);
  server.listen(0, "127.0.0.1", resolve);
});

let execution;
try {
  const address = server.address();
  if (!address || typeof address === "string") {
    throw new Error("HTTPYAC validation server did not expose a TCP port");
  }
  execution = await runHTTPYAC(address.port);
} finally {
  await new Promise((resolve) => server.close(resolve));
}
if (execution.code !== 0) {
  process.stderr.write(execution.stdout);
  process.stderr.write(execution.stderr);
  failures.push(`HTTPYAC parser/executor exited with ${execution.code}`);
}
if (receivedRequests !== requests.length) {
  failures.push(
    `HTTPYAC executed ${receivedRequests} request(s), expected ${requests.length}`,
  );
}
if (failures.length > 0) {
  reportFailures(failures);
}

console.log(
  `${filenames.length} HTTPYAC file(s) parsed and executed safely; ` +
    `${operations.length} OpenAPI operation(s), ${requests.length} request(s), ` +
    "negative boundaries, anonymous isolation, and environment placeholders passed.",
);

function normalizeRequestPath(rawURL) {
  const withoutBase = rawURL.replace(/^\{\{apiBaseUrl\}\}/u, "");
  return withoutBase
    .split("?", 1)[0]
    .replace(/\{\{([A-Za-z][A-Za-z0-9]*)\}\}/gu, "{$1}");
}

function pathsMatch(contractPath, requestPath) {
  const contractSegments = contractPath.split("/");
  const requestSegments = requestPath.split("/");
  if (contractSegments.length !== requestSegments.length) {
    return false;
  }
  return contractSegments.every(
    (segment, index) =>
      /^\{[^}]+\}$/u.test(segment) || segment === requestSegments[index],
  );
}

function runHTTPYAC(port) {
  return new Promise((resolve, reject) => {
    const child = spawn(
      "pnpm",
      [
        "exec",
        "httpyac",
        "send",
        "http/*.http",
        "--all",
        "--env",
        "ci",
        "--var",
        `apiBaseUrl=http://127.0.0.1:${port}`,
        "--timeout",
        "2000",
        "--output",
        "none",
        "--output-failed",
        "short",
        "--no-color",
        "--bail",
      ],
      {
        cwd: repositoryRoot,
        stdio: ["ignore", "pipe", "pipe"],
      },
    );
    let stdout = "";
    let stderr = "";
    child.stdout.on("data", (chunk) => {
      stdout += chunk;
    });
    child.stderr.on("data", (chunk) => {
      stderr += chunk;
    });
    child.once("error", reject);
    child.once("close", (code) => {
      resolve({ code, stderr, stdout });
    });
  });
}

function reportFailures(items) {
  console.error("HTTPYAC contract validation failed:");
  for (const failure of items) {
    console.error(`- ${failure}`);
  }
  process.exit(1);
}
