import { spawnSync } from "node:child_process";
import path from "node:path";
import process from "node:process";

import budgets from "../../config/quality-budgets.json" with { type: "json" };

const repositoryRoot = path.resolve(import.meta.dirname, "../..");
const benchmarkNames = Object.keys(budgets.goBenchmarks);
const benchmarkPattern = `^(${benchmarkNames.join("|")})$`;
const result = spawnSync(
  "go",
  [
    "-C",
    path.join(repositoryRoot, "apps/api"),
    "test",
    "-run",
    "^$",
    "-bench",
    benchmarkPattern,
    "-benchtime=100x",
    "-benchmem",
    "-count=3",
    "./internal/domain/issue",
    "./internal/domain/profile",
  ],
  {
    encoding: "utf8",
    env: process.env,
  },
);

process.stdout.write(result.stdout);
process.stderr.write(result.stderr);
if (result.error) {
  throw result.error;
}
if (result.status !== 0) {
  process.exit(result.status ?? 1);
}

const measurements = new Map();
const benchmarkLine =
  /^(Benchmark\w+)-\d+\s+\d+\s+([\d.]+)\s+ns\/op\s+(\d+)\s+B\/op\s+(\d+)\s+allocs\/op$/;
for (const line of result.stdout.split("\n")) {
  const match = benchmarkLine.exec(line.trim());
  if (!match) {
    continue;
  }
  const [, name, nanoseconds, bytes, allocations] = match;
  const samples = measurements.get(name) ?? [];
  samples.push({
    allocations: Number(allocations),
    bytes: Number(bytes),
    nanoseconds: Number(nanoseconds),
  });
  measurements.set(name, samples);
}

const failures = [];
for (const name of benchmarkNames) {
  const samples = measurements.get(name);
  if (!samples || samples.length !== 3) {
    failures.push(`${name}: expected 3 benchmark samples`);
    continue;
  }
  const worst = {
    allocations: Math.max(...samples.map((sample) => sample.allocations)),
    bytes: Math.max(...samples.map((sample) => sample.bytes)),
    nanoseconds: Math.max(...samples.map((sample) => sample.nanoseconds)),
  };
  const budget = budgets.goBenchmarks[name];
  console.log(
    `${name}: worst ${worst.nanoseconds} ns/op, ${worst.bytes} B/op, ` +
      `${worst.allocations} allocs/op`,
  );
  if (worst.nanoseconds > budget.maximumNanosecondsPerOperation) {
    failures.push(
      `${name}: ${worst.nanoseconds} ns/op exceeds ` +
        `${budget.maximumNanosecondsPerOperation}`,
    );
  }
  if (worst.bytes > budget.maximumBytesPerOperation) {
    failures.push(
      `${name}: ${worst.bytes} B/op exceeds ${budget.maximumBytesPerOperation}`,
    );
  }
  if (worst.allocations > budget.maximumAllocationsPerOperation) {
    failures.push(
      `${name}: ${worst.allocations} allocs/op exceeds ` +
        `${budget.maximumAllocationsPerOperation}`,
    );
  }
}

if (failures.length > 0) {
  console.error("Go performance budget failed:");
  for (const failure of failures) {
    console.error(`- ${failure}`);
  }
  process.exit(1);
}
