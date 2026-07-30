import { readFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";

import Ajv2020 from "ajv/dist/2020.js";
import addFormats from "ajv-formats";
import { parse } from "yaml";

const repositoryRoot = path.resolve(import.meta.dirname, "../..");
const contractRoot = path.join(repositoryRoot, "packages/contracts");
const specification = parse(
  await readFile(path.join(contractRoot, "openapi.yaml"), "utf8"),
);
const manifest = JSON.parse(
  await readFile(path.join(contractRoot, "fixtures/manifest.json"), "utf8"),
);
const ajv = new Ajv2020({
  allErrors: true,
  allowUnionTypes: true,
  strict: false,
});
addFormats(ajv);

const failures = [];
for (const entry of manifest.fixtures) {
  const fixturePath = path.join(contractRoot, "fixtures", entry.file);
  const fixture = JSON.parse(await readFile(fixturePath, "utf8"));
  const validate = ajv.compile({
    $ref: `#/components/schemas/${entry.schema}`,
    components: specification.components,
  });
  if (!validate(fixture)) {
    failures.push(
      `${entry.file}: ${ajv.errorsText(validate.errors, {
        dataVar: entry.file,
        separator: "\n  ",
      })}`,
    );
    continue;
  }

  const unknownEnvelope = structuredClone(fixture);
  unknownEnvelope.undocumentedContractField = true;
  if (validate(unknownEnvelope)) {
    failures.push(`${entry.file}: accepts an undocumented envelope field`);
  }

  const payloadKey = Object.hasOwn(fixture, "data") ? "data" : "error";
  if (
    fixture[payloadKey] !== null &&
    typeof fixture[payloadKey] === "object" &&
    !Array.isArray(fixture[payloadKey])
  ) {
    const unknownPayload = structuredClone(fixture);
    unknownPayload[payloadKey].undocumentedContractField = true;
    if (validate(unknownPayload)) {
      failures.push(
        `${entry.file}: accepts an undocumented ${payloadKey} field`,
      );
    }
  }

  const missingMetadata = structuredClone(fixture);
  delete missingMetadata.meta;
  if (validate(missingMetadata)) {
    failures.push(`${entry.file}: accepts an envelope without metadata`);
  }

  console.log(
    `${entry.file}: ${entry.schema} valid and strict-envelope mutations rejected`,
  );
}

if (failures.length > 0) {
  console.error("Contract fixture validation failed:");
  for (const failure of failures) {
    console.error(`- ${failure}`);
  }
  process.exit(1);
}
