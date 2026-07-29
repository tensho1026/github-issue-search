import { readFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";

import budgets from "../../config/quality-budgets.json" with { type: "json" };

const summaryPath = path.resolve(
  import.meta.dirname,
  "../../apps/web/coverage/coverage-summary.json",
);
const summary = JSON.parse(await readFile(summaryPath, "utf8"));
const failures = [];

for (const metric of ["branches", "functions", "lines", "statements"]) {
  const actual = summary.total[metric].pct;
  const required = budgets.coverage.web[metric];
  console.log(`Web ${metric} coverage: ${actual}% (required ${required}%)`);
  if (actual < required) {
    failures.push(`${metric}: ${actual}% is below ${required}%`);
  }
}

if (failures.length > 0) {
  console.error("Web coverage budget failed:");
  for (const failure of failures) {
    console.error(`- ${failure}`);
  }
  process.exit(1);
}
