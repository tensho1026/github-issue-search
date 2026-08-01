import { execFile } from "node:child_process";
import { createHash } from "node:crypto";
import { readFile, readdir } from "node:fs/promises";
import path from "node:path";
import process from "node:process";
import { promisify } from "node:util";

const execute = promisify(execFile);
const repositoryRoot = path.resolve(import.meta.dirname, "../..");
const migrationRoot = path.join(
  repositoryRoot,
  "apps/api/internal/database/postgres/migrations",
);
const migrationPrefix = "apps/api/internal/database/postgres/migrations/";
const filenamePattern = /^(\d{6})_[a-z0-9_]+\.sql$/;
const failures = [];
const entries = await readdir(migrationRoot, { withFileTypes: true });
const filenames = entries.map((entry) => entry.name).sort();

for (const entry of entries) {
  if (!entry.isFile() || !filenamePattern.test(entry.name)) {
    failures.push(`${entry.name}: expected a six-digit forward-only .sql file`);
  }
}

for (const [index, filename] of filenames.entries()) {
  const match = filenamePattern.exec(filename);
  if (!match) {
    continue;
  }
  const expectedVersion = String(index + 1).padStart(6, "0");
  if (match[1] !== expectedVersion) {
    failures.push(
      `${filename}: expected contiguous version ${expectedVersion}`,
    );
  }
  const source = await readFile(path.join(migrationRoot, filename), "utf8");
  if (!source.endsWith("\n")) {
    failures.push(`${filename}: must end with a newline`);
  }
  if (Buffer.byteLength(source) > 64 * 1024) {
    failures.push(`${filename}: exceeds the 64 KiB review limit`);
  }
  const checks = [
    [/\b(?:BEGIN|COMMIT)\s*;/iu, "runner-owned transaction statement"],
    [/\bDROP\s+(?:TABLE|SCHEMA|DATABASE)\b/iu, "destructive DROP"],
    [/\bTRUNCATE\b/iu, "destructive TRUNCATE"],
    [/\bALTER\s+TABLE\b[\s\S]*\bDROP\b/iu, "destructive ALTER DROP"],
    [/\bCREATE\s+EXTENSION\b/iu, "privileged extension installation"],
    [/\bGRANT\b[\s\S]*\bPUBLIC\b/iu, "grant to PUBLIC"],
    [/\bpostgres(?:ql)?:\/\//iu, "database connection URL"],
  ];
  for (const [pattern, label] of checks) {
    if (pattern.test(source)) {
      failures.push(`${filename}: contains forbidden ${label}`);
    }
  }
  const checksum = createHash("sha256").update(source).digest("hex");
  console.log(`${filename} sha256:${checksum}`);
}

const combinedSource = (
  await Promise.all(
    filenames
      .filter((filename) => filenamePattern.test(filename))
      .map((filename) => readFile(path.join(migrationRoot, filename), "utf8")),
  )
).join("\n");
for (const table of [
  "auth_sessions",
  "bookmarks",
  "github_identities",
  "saved_searches",
  "user_preferences",
]) {
  const tablePattern = new RegExp(
    `CREATE TABLE ${table} \\([\\s\\S]*account_id uuid[^;]*REFERENCES accounts\\(id\\) ON DELETE CASCADE`,
    "u",
  );
  if (!tablePattern.test(combinedSource)) {
    failures.push(
      `${table}: must declare account ownership with ON DELETE CASCADE`,
    );
  }
}

const [baseRevision, headRevision] = process.argv
  .slice(2)
  .filter((argument) => argument !== "--");
if (baseRevision && headRevision) {
  await enforceAppendOnlyHistory(baseRevision, headRevision);
}

if (failures.length > 0) {
  console.error("Database migration policy violations:");
  for (const failure of failures) {
    console.error(`- ${failure}`);
  }
  process.exit(1);
}

console.log(`${filenames.length} forward-only migration(s) passed policy.`);

async function enforceAppendOnlyHistory(baseRevision, headRevision) {
  let output;
  try {
    ({ stdout: output } = await execute(
      "git",
      [
        "diff",
        "--name-status",
        "--find-renames",
        baseRevision,
        headRevision,
        "--",
        migrationPrefix,
      ],
      { cwd: repositoryRoot },
    ));
  } catch {
    failures.push("could not compare migration history revisions");
    return;
  }
  const baseFiles = await filesAtRevision(baseRevision);
  const baseVersions = baseFiles
    .map((filename) => filenamePattern.exec(path.basename(filename))?.[1])
    .filter(Boolean)
    .map(Number);
  const maximumBaseVersion =
    baseVersions.length > 0 ? Math.max(...baseVersions) : 0;

  for (const line of output.trim().split("\n").filter(Boolean)) {
    const [status, firstPath, secondPath] = line.split("\t");
    if (status === "A") {
      const match = filenamePattern.exec(path.basename(firstPath));
      if (match && Number(match[1]) <= maximumBaseVersion) {
        failures.push(
          `${firstPath}: new migrations must follow version ${String(
            maximumBaseVersion,
          ).padStart(6, "0")}`,
        );
      }
      continue;
    }
    failures.push(
      `${firstPath}${secondPath ? ` -> ${secondPath}` : ""}: applied migration files are append-only`,
    );
  }
}

async function filesAtRevision(revision) {
  try {
    const { stdout } = await execute(
      "git",
      ["ls-tree", "-r", "--name-only", revision, "--", migrationPrefix],
      { cwd: repositoryRoot },
    );
    return stdout.trim().split("\n").filter(Boolean);
  } catch {
    failures.push(`could not inspect migration catalog at ${revision}`);
    return [];
  }
}
