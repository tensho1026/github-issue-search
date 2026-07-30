import type {
  OssCategory,
  RepositoryDiscoveryRequest,
  SupportedSpdxLicense,
} from "../../../shared/api/generated";

export type JapaneseReadmeFilter = "any" | "no" | "yes";
export type ForkPolicy = NonNullable<RepositoryDiscoveryRequest["forkPolicy"]>;

export type RepositoryFilters = {
  categories: OssCategory[];
  excludeArchived: boolean;
  forkPolicy: ForkPolicy;
  hasJapaneseReadme: JapaneseReadmeFilter;
  languages: string[];
  licenses: SupportedSpdxLicense[];
  maximumDifficulty: number;
  maximumOpenIssues: number;
  minimumForks: number;
  minimumOpenIssues: number;
  minimumReadiness: number;
  minimumStars: number;
  page: number;
  perPage: number;
  technologies: string[];
  updatedWithinDays: number;
};

export type RepositoryFilterErrors = Partial<
  Record<keyof RepositoryFilters | "form", string>
>;

export type DecodedRepositoryLocation = {
  errors: RepositoryFilterErrors;
  filters: RepositoryFilters;
  shouldSearch: boolean;
  valid: boolean;
};

type FilterOption<TValue extends string | number = string> = Readonly<{
  label: string;
  value: TValue;
}>;

const maximumFilterValues = 10;
const maximumFilterCharacters = 64;
const maximumFilterBytes = 128;
const maximumCount = 10_000_000;
const maximumUpdatedWithinDays = 3650;
const maximumPage = 50;
const maximumPageSize = 50;

const licenses = [
  "0BSD",
  "AGPL-3.0",
  "Apache-2.0",
  "BSD-2-Clause",
  "BSD-3-Clause",
  "BSL-1.0",
  "CC0-1.0",
  "EPL-2.0",
  "GPL-2.0",
  "GPL-3.0",
  "ISC",
  "LGPL-2.1",
  "LGPL-3.0",
  "MIT",
  "MPL-2.0",
  "Unlicense",
] as const satisfies readonly SupportedSpdxLicense[];

const categories = [
  "application",
  "data",
  "documentation",
  "education",
  "framework",
  "infrastructure",
  "library",
  "security",
  "tooling",
] as const satisfies readonly OssCategory[];

export const repositoryFilterOptions = Object.freeze({
  categories: categories.map((value) => ({
    label: titleCase(value),
    value,
  })) satisfies Array<FilterOption<OssCategory>>,
  difficulties: [
    { label: "1 · Very low", value: 1 },
    { label: "2 · Low", value: 2 },
    { label: "3 · Medium", value: 3 },
    { label: "4 · High", value: 4 },
    { label: "5 · Very high", value: 5 },
  ] satisfies Array<FilterOption<number>>,
  forkPolicies: [
    { label: "Original repositories", value: "exclude" },
    { label: "Originals and forks", value: "include" },
    { label: "Forks only", value: "only" },
  ] satisfies Array<FilterOption<ForkPolicy>>,
  japaneseReadmes: [
    { label: "Any README evidence", value: "any" },
    { label: "Japanese detected", value: "yes" },
    { label: "Analyzed, not Japanese", value: "no" },
  ] satisfies Array<FilterOption<JapaneseReadmeFilter>>,
  languages: [
    { label: "TypeScript", value: "TypeScript" },
    { label: "JavaScript", value: "JavaScript" },
    { label: "Go", value: "Go" },
    { label: "Python", value: "Python" },
    { label: "Rust", value: "Rust" },
    { label: "Java", value: "Java" },
    { label: "Kotlin", value: "Kotlin" },
    { label: "C#", value: "C#" },
    { label: "Ruby", value: "Ruby" },
    { label: "PHP", value: "PHP" },
  ] satisfies Array<FilterOption>,
  licenses: licenses.map((value) => ({
    label: value,
    value,
  })) satisfies Array<FilterOption<SupportedSpdxLicense>>,
  pageSizes: [
    { label: "10 per page", value: 10 },
    { label: "20 per page", value: 20 },
    { label: "50 per page", value: 50 },
  ] satisfies Array<FilterOption<number>>,
  technologies: [
    { label: "React", value: "React" },
    { label: "Next.js", value: "Next.js" },
    { label: "Vue", value: "Vue" },
    { label: "Angular", value: "Angular" },
    { label: "Svelte", value: "Svelte" },
    { label: "Django", value: "Django" },
    { label: "FastAPI", value: "FastAPI" },
    { label: "Gin", value: "Gin" },
    { label: "Kubernetes", value: "Kubernetes" },
    { label: "Terraform", value: "Terraform" },
  ] satisfies Array<FilterOption>,
});

export const repositoryFilterDescriptions = Object.freeze({
  categories:
    "Repositories matching any selected explainable category are considered.",
  languages:
    "Repositories matching any selected primary language are considered.",
  licenses:
    "Repositories matching any selected supported SPDX identifier are considered.",
  technologies:
    "Every selected term must appear in public topics or bounded README evidence.",
});

export function createDefaultRepositoryFilters(): RepositoryFilters {
  return {
    categories: [],
    excludeArchived: true,
    forkPolicy: "exclude",
    hasJapaneseReadme: "any",
    languages: [],
    licenses: [],
    maximumDifficulty: 3,
    maximumOpenIssues: 500,
    minimumForks: 0,
    minimumOpenIssues: 1,
    minimumReadiness: 40,
    minimumStars: 10,
    page: 1,
    perPage: 20,
    technologies: [],
    updatedWithinDays: 365,
  };
}

const parameterNames = Object.freeze({
  categories: "category",
  excludeArchived: "excludeArchived",
  forkPolicy: "forkPolicy",
  hasJapaneseReadme: "japaneseReadme",
  languages: "language",
  licenses: "license",
  maximumDifficulty: "maximumDifficulty",
  maximumOpenIssues: "maximumOpenIssues",
  minimumForks: "minimumForks",
  minimumOpenIssues: "minimumOpenIssues",
  minimumReadiness: "minimumReadiness",
  minimumStars: "minimumStars",
  page: "page",
  perPage: "perPage",
  run: "search",
  technologies: "technology",
  updatedWithinDays: "updatedWithinDays",
});

const allowedParameterNames = new Set<string>(Object.values(parameterNames));

export function validateRepositoryFilters(
  filters: RepositoryFilters,
): RepositoryFilterErrors {
  const errors: RepositoryFilterErrors = {};
  validateFilterValues("languages", filters.languages, errors);
  validateFilterValues("technologies", filters.technologies, errors);
  validateEnumValues(
    "licenses",
    filters.licenses,
    licenses,
    "SPDX license",
    errors,
  );
  validateEnumValues(
    "categories",
    filters.categories,
    categories,
    "OSS category",
    errors,
  );
  validateCount("minimumStars", filters.minimumStars, errors);
  validateCount("minimumForks", filters.minimumForks, errors);
  validateCount("minimumOpenIssues", filters.minimumOpenIssues, errors);
  validateCount("maximumOpenIssues", filters.maximumOpenIssues, errors);
  if (filters.maximumOpenIssues < filters.minimumOpenIssues) {
    errors.maximumOpenIssues =
      "Maximum open issues must be at least the minimum open issues.";
  }
  if (
    !Number.isSafeInteger(filters.updatedWithinDays) ||
    filters.updatedWithinDays < 1 ||
    filters.updatedWithinDays > maximumUpdatedWithinDays
  ) {
    errors.updatedWithinDays = `Recency must be between 1 and ${maximumUpdatedWithinDays} days.`;
  }
  if (
    !Number.isSafeInteger(filters.maximumDifficulty) ||
    filters.maximumDifficulty < 1 ||
    filters.maximumDifficulty > 5
  ) {
    errors.maximumDifficulty = "Difficulty must be between 1 and 5.";
  }
  if (
    !Number.isSafeInteger(filters.minimumReadiness) ||
    filters.minimumReadiness < 0 ||
    filters.minimumReadiness > 100
  ) {
    errors.minimumReadiness = "Readiness must be between 0 and 100.";
  }
  if (
    !repositoryFilterOptions.forkPolicies.some(
      (option) => option.value === filters.forkPolicy,
    )
  ) {
    errors.forkPolicy = "Fork policy is not supported.";
  }
  if (
    !repositoryFilterOptions.japaneseReadmes.some(
      (option) => option.value === filters.hasJapaneseReadme,
    )
  ) {
    errors.hasJapaneseReadme = "Japanese README selection is not supported.";
  }
  if (
    !Number.isSafeInteger(filters.page) ||
    filters.page < 1 ||
    filters.page > maximumPage
  ) {
    errors.page = `Page must be between 1 and ${maximumPage}.`;
  }
  if (
    !Number.isSafeInteger(filters.perPage) ||
    filters.perPage < 1 ||
    filters.perPage > maximumPageSize
  ) {
    errors.perPage = `Page size must be between 1 and ${maximumPageSize}.`;
  }
  return errors;
}

export function decodeRepositorySearchParams(
  parameters: URLSearchParams,
): DecodedRepositoryLocation {
  const defaults = createDefaultRepositoryFilters();
  const locationErrors: string[] = [];
  for (const name of new Set(parameters.keys())) {
    if (!allowedParameterNames.has(name)) {
      locationErrors.push(`"${name}" is not a supported filter.`);
    }
  }
  const forkPolicy = readScalar(
    parameters,
    parameterNames.forkPolicy,
    defaults.forkPolicy,
    locationErrors,
  ) as ForkPolicy;
  const hasJapaneseReadme = readScalar(
    parameters,
    parameterNames.hasJapaneseReadme,
    defaults.hasJapaneseReadme,
    locationErrors,
  ) as JapaneseReadmeFilter;
  const filters: RepositoryFilters = {
    categories: readList(
      parameters,
      parameterNames.categories,
      defaults.categories,
    ),
    excludeArchived: readBoolean(
      parameters,
      parameterNames.excludeArchived,
      defaults.excludeArchived,
      locationErrors,
    ),
    forkPolicy,
    hasJapaneseReadme,
    languages: readList(
      parameters,
      parameterNames.languages,
      defaults.languages,
    ),
    licenses: readList(parameters, parameterNames.licenses, defaults.licenses),
    maximumDifficulty: readInteger(
      parameters,
      parameterNames.maximumDifficulty,
      defaults.maximumDifficulty,
      locationErrors,
    ),
    maximumOpenIssues: readInteger(
      parameters,
      parameterNames.maximumOpenIssues,
      defaults.maximumOpenIssues,
      locationErrors,
    ),
    minimumForks: readInteger(
      parameters,
      parameterNames.minimumForks,
      defaults.minimumForks,
      locationErrors,
    ),
    minimumOpenIssues: readInteger(
      parameters,
      parameterNames.minimumOpenIssues,
      defaults.minimumOpenIssues,
      locationErrors,
    ),
    minimumReadiness: readInteger(
      parameters,
      parameterNames.minimumReadiness,
      defaults.minimumReadiness,
      locationErrors,
    ),
    minimumStars: readInteger(
      parameters,
      parameterNames.minimumStars,
      defaults.minimumStars,
      locationErrors,
    ),
    page: readInteger(
      parameters,
      parameterNames.page,
      defaults.page,
      locationErrors,
    ),
    perPage: readInteger(
      parameters,
      parameterNames.perPage,
      defaults.perPage,
      locationErrors,
    ),
    technologies: readList(
      parameters,
      parameterNames.technologies,
      defaults.technologies,
    ),
    updatedWithinDays: readInteger(
      parameters,
      parameterNames.updatedWithinDays,
      defaults.updatedWithinDays,
      locationErrors,
    ),
  };
  const searchValues = parameters.getAll(parameterNames.run);
  const shouldSearch = searchValues.length === 1 && searchValues[0] === "1";
  if (
    searchValues.length > 1 ||
    (searchValues.length === 1 && searchValues[0] !== "1")
  ) {
    locationErrors.push(`"${parameterNames.run}" must be exactly "1".`);
  }
  const errors = validateRepositoryFilters(filters);
  if (locationErrors.length > 0) {
    errors.form = `The shared repository URL is invalid: ${locationErrors.join(
      " ",
    )}`;
  }
  return {
    errors,
    filters,
    shouldSearch,
    valid: Object.keys(errors).length === 0,
  };
}

export function encodeRepositorySearchParams(
  filters: RepositoryFilters,
  shouldSearch = true,
): URLSearchParams {
  const normalized = normalizeRepositoryFilters(filters);
  if (Object.keys(validateRepositoryFilters(normalized)).length > 0) {
    throw new Error("Cannot encode invalid repository discovery filters.");
  }
  const parameters = new URLSearchParams();
  appendList(parameters, parameterNames.languages, normalized.languages);
  appendList(parameters, parameterNames.technologies, normalized.technologies);
  appendList(parameters, parameterNames.licenses, normalized.licenses);
  appendList(parameters, parameterNames.categories, normalized.categories);
  parameters.set(
    parameterNames.minimumStars,
    normalized.minimumStars.toString(),
  );
  parameters.set(
    parameterNames.minimumForks,
    normalized.minimumForks.toString(),
  );
  parameters.set(
    parameterNames.minimumOpenIssues,
    normalized.minimumOpenIssues.toString(),
  );
  parameters.set(
    parameterNames.maximumOpenIssues,
    normalized.maximumOpenIssues.toString(),
  );
  parameters.set(
    parameterNames.updatedWithinDays,
    normalized.updatedWithinDays.toString(),
  );
  parameters.set(
    parameterNames.maximumDifficulty,
    normalized.maximumDifficulty.toString(),
  );
  parameters.set(
    parameterNames.minimumReadiness,
    normalized.minimumReadiness.toString(),
  );
  parameters.set(
    parameterNames.hasJapaneseReadme,
    normalized.hasJapaneseReadme,
  );
  parameters.set(parameterNames.forkPolicy, normalized.forkPolicy);
  parameters.set(
    parameterNames.excludeArchived,
    normalized.excludeArchived.toString(),
  );
  parameters.set(parameterNames.page, normalized.page.toString());
  parameters.set(parameterNames.perPage, normalized.perPage.toString());
  if (shouldSearch) {
    parameters.set(parameterNames.run, "1");
  }
  return parameters;
}

export function normalizeRepositoryFilters(
  filters: RepositoryFilters,
): RepositoryFilters {
  return {
    ...filters,
    categories: normalizeList(filters.categories),
    languages: normalizeList(filters.languages),
    licenses: normalizeList(filters.licenses),
    technologies: normalizeList(filters.technologies),
  };
}

export function toRepositoryDiscoveryRequest(
  filters: RepositoryFilters,
): RepositoryDiscoveryRequest {
  const normalized = normalizeRepositoryFilters(filters);
  const request: RepositoryDiscoveryRequest = {
    categories: normalized.categories,
    excludeArchived: normalized.excludeArchived,
    forkPolicy: normalized.forkPolicy,
    languages: normalized.languages,
    licenses: normalized.licenses,
    maximumDifficulty: normalized.maximumDifficulty,
    maximumOpenIssues: normalized.maximumOpenIssues,
    minimumForks: normalized.minimumForks,
    minimumOpenIssues: normalized.minimumOpenIssues,
    minimumReadiness: normalized.minimumReadiness,
    minimumStars: normalized.minimumStars,
    technologies: normalized.technologies,
    updatedWithinDays: normalized.updatedWithinDays,
  };
  if (normalized.hasJapaneseReadme !== "any") {
    request.hasJapaneseReadme = normalized.hasJapaneseReadme === "yes";
  }
  return request;
}

function validateFilterValues(
  field: "languages" | "technologies",
  values: string[],
  errors: RepositoryFilterErrors,
) {
  if (values.length > maximumFilterValues) {
    errors[field] = `Choose at most ${maximumFilterValues} values.`;
    return;
  }
  const normalized = normalizeList(values);
  if (normalized.length !== values.length) {
    errors[field] = "Values must be unique and non-empty.";
    return;
  }
  const invalid = normalized.some((value) => {
    const bytes = new TextEncoder().encode(value).byteLength;
    const hasForbiddenCharacter = [...value].some((character) => {
      const codePoint = character.codePointAt(0) ?? 0;
      return (
        character === '"' ||
        character === "\\" ||
        codePoint <= 31 ||
        codePoint === 127
      );
    });
    return (
      [...value].length > maximumFilterCharacters ||
      bytes > maximumFilterBytes ||
      hasForbiddenCharacter
    );
  });
  if (invalid) {
    errors[field] =
      "Values must be 1–64 characters and cannot contain quotes, backslashes, or control characters.";
  }
}

function validateEnumValues<TValue extends string>(
  field: "categories" | "licenses",
  values: TValue[],
  supportedValues: readonly TValue[],
  label: string,
  errors: RepositoryFilterErrors,
) {
  if (values.length > maximumFilterValues) {
    errors[field] = `Choose at most ${maximumFilterValues} values.`;
    return;
  }
  const supported = new Set<string>(supportedValues);
  const normalized = normalizeList(values);
  if (
    normalized.length !== values.length ||
    normalized.some((value) => !supported.has(value))
  ) {
    errors[field] = `Choose unique, supported ${label} values.`;
  }
}

function validateCount(
  field:
    "maximumOpenIssues" | "minimumForks" | "minimumOpenIssues" | "minimumStars",
  value: number,
  errors: RepositoryFilterErrors,
) {
  if (!Number.isSafeInteger(value) || value < 0 || value > maximumCount) {
    errors[field] =
      "Count must be a non-negative integer no greater than 10,000,000.";
  }
}

function normalizeList<TValue extends string>(
  values: readonly TValue[],
): TValue[] {
  const normalized = new Map<string, TValue>();
  for (const rawValue of values) {
    const value = rawValue.trim() as TValue;
    const key = value.toLocaleLowerCase("en");
    if (value && !normalized.has(key)) {
      normalized.set(key, value);
    }
  }
  return [...normalized.values()].sort((left, right) =>
    left.localeCompare(right, "en", { sensitivity: "base" }),
  );
}

function readScalar(
  parameters: URLSearchParams,
  name: string,
  fallback: string,
  errors: string[],
): string {
  const values = parameters.getAll(name);
  if (values.length === 0) {
    return fallback;
  }
  if (values.length !== 1) {
    errors.push(`"${name}" must be provided once.`);
    return fallback;
  }
  return values[0] ?? fallback;
}

function readInteger(
  parameters: URLSearchParams,
  name: string,
  fallback: number,
  errors: string[],
): number {
  const rawValue = readScalar(parameters, name, "", errors);
  if (rawValue === "") {
    return fallback;
  }
  if (!/^(?:0|[1-9]\d*)$/u.test(rawValue)) {
    errors.push(`"${name}" must be an integer.`);
    return fallback;
  }
  const value = Number(rawValue);
  if (!Number.isSafeInteger(value)) {
    errors.push(`"${name}" is outside the supported integer range.`);
    return fallback;
  }
  return value;
}

function readBoolean(
  parameters: URLSearchParams,
  name: string,
  fallback: boolean,
  errors: string[],
): boolean {
  const rawValue = readScalar(parameters, name, "", errors);
  if (rawValue === "") {
    return fallback;
  }
  if (rawValue !== "true" && rawValue !== "false") {
    errors.push(`"${name}" must be "true" or "false".`);
    return fallback;
  }
  return rawValue === "true";
}

function readList<TValue extends string>(
  parameters: URLSearchParams,
  name: string,
  fallback: TValue[],
): TValue[] {
  const values = parameters.getAll(name) as TValue[];
  return values.length === 0 ? [...fallback] : values;
}

function appendList(
  parameters: URLSearchParams,
  name: string,
  values: readonly string[],
) {
  for (const value of values) {
    parameters.append(name, value);
  }
}

function titleCase(value: string): string {
  return value.charAt(0).toUpperCase() + value.slice(1).replaceAll("_", " ");
}
