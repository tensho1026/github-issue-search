import type { IssueSearchRequest } from "../../../shared/api/generated";
import { validateGitHubUsername } from "../../../shared/lib/github-username";

export type EffortBand = NonNullable<IssueSearchRequest["maximumEffort"]>;

export type SearchFilters = {
  username: string;
  languages: string[];
  frameworks: string[];
  labels: string[];
  minimumStars: number;
  maximumDifficulty: number;
  maximumEffort: EffortBand | "";
  updatedWithinDays: number;
  includeDocumentation: boolean;
  includeEnglish: boolean;
  excludeArchived: boolean;
  page: number;
  perPage: number;
};

export type SearchFilterErrors = Partial<
  Record<keyof SearchFilters | "form", string>
>;

export type DecodedSearchLocation = {
  errors: SearchFilterErrors;
  filters: SearchFilters;
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
const maximumUpdatedWithinDays = 3650;
const maximumPageSize = 50;

export const searchFilterOptions = Object.freeze({
  difficulties: [
    { label: "1 · Starter", value: 1 },
    { label: "2 · Approachable", value: 2 },
    { label: "3 · Intermediate", value: 3 },
    { label: "4 · Advanced", value: 4 },
    { label: "5 · Expert", value: 5 },
  ] satisfies Array<FilterOption<number>>,
  efforts: [
    { label: "Any available time", value: "" },
    { label: "Up to 30 minutes", value: "thirty_minutes" },
    { label: "Up to 2 hours", value: "two_hours" },
    { label: "Up to half a day", value: "half_day" },
    { label: "Up to one day", value: "one_day" },
    { label: "Up to three days", value: "three_days" },
  ] satisfies Array<FilterOption<EffortBand | "">>,
  frameworks: [
    { label: "React", value: "React" },
    { label: "Next.js", value: "Next.js" },
    { label: "Vue", value: "Vue" },
    { label: "Angular", value: "Angular" },
    { label: "Svelte", value: "Svelte" },
    { label: "Express", value: "Express" },
    { label: "NestJS", value: "NestJS" },
    { label: "Django", value: "Django" },
    { label: "FastAPI", value: "FastAPI" },
    { label: "Gin", value: "Gin" },
  ] satisfies Array<FilterOption>,
  labels: [
    { label: "good first issue", value: "good first issue" },
    { label: "help wanted", value: "help wanted" },
    { label: "bug", value: "bug" },
    { label: "enhancement", value: "enhancement" },
    { label: "documentation", value: "documentation" },
    { label: "accessibility", value: "accessibility" },
    { label: "tests", value: "tests" },
    { label: "performance", value: "performance" },
    { label: "beginner", value: "beginner" },
    { label: "up-for-grabs", value: "up-for-grabs" },
  ] satisfies Array<FilterOption>,
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
  pageSizes: [
    { label: "10 per page", value: 10 },
    { label: "20 per page", value: 20 },
    { label: "50 per page", value: 50 },
  ] satisfies Array<FilterOption<number>>,
});

export const searchFilterDescriptions = Object.freeze({
  frameworks:
    "All selected framework terms must appear in the issue or repository discovery text.",
  labels:
    "Issues matching any selected label are considered. Documentation can also be included below.",
  languages:
    "Repositories matching any selected primary language are considered.",
});

const defaultLabels = ["good first issue", "help wanted"];

export function createDefaultSearchFilters(username = ""): SearchFilters {
  return {
    excludeArchived: true,
    frameworks: [],
    includeDocumentation: false,
    includeEnglish: true,
    labels: [...defaultLabels],
    languages: [],
    maximumDifficulty: 3,
    maximumEffort: "",
    minimumStars: 10,
    page: 1,
    perPage: 20,
    updatedWithinDays: 180,
    username,
  };
}

const parameterNames = Object.freeze({
  excludeArchived: "excludeArchived",
  frameworks: "framework",
  includeDocumentation: "includeDocumentation",
  includeEnglish: "includeEnglish",
  labels: "label",
  languages: "language",
  maximumDifficulty: "maximumDifficulty",
  maximumEffort: "maximumEffort",
  minimumStars: "minimumStars",
  page: "page",
  perPage: "perPage",
  run: "search",
  updatedWithinDays: "updatedWithinDays",
  username: "username",
});

export function validateSearchFilters(
  filters: SearchFilters,
): SearchFilterErrors {
  const errors: SearchFilterErrors = {};
  const username = validateGitHubUsername(filters.username);
  if (!username.valid) {
    errors.username = username.message;
  }
  validateFilterValues("languages", filters.languages, errors);
  validateFilterValues("frameworks", filters.frameworks, errors);
  validateFilterValues("labels", filters.labels, errors);

  if (!Number.isSafeInteger(filters.minimumStars) || filters.minimumStars < 0) {
    errors.minimumStars = "Minimum stars must be a non-negative integer.";
  }
  if (
    !Number.isSafeInteger(filters.maximumDifficulty) ||
    filters.maximumDifficulty < 1 ||
    filters.maximumDifficulty > 5
  ) {
    errors.maximumDifficulty = "Difficulty must be between 1 and 5.";
  }
  if (
    filters.maximumEffort !== "" &&
    !searchFilterOptions.efforts.some(
      (option) => option.value === filters.maximumEffort,
    )
  ) {
    errors.maximumEffort = "Available time is not a supported effort band.";
  }
  if (
    !Number.isSafeInteger(filters.updatedWithinDays) ||
    filters.updatedWithinDays < 1 ||
    filters.updatedWithinDays > maximumUpdatedWithinDays
  ) {
    errors.updatedWithinDays = `Recency must be between 1 and ${maximumUpdatedWithinDays} days.`;
  }
  if (!Number.isSafeInteger(filters.page) || filters.page < 1) {
    errors.page = "Page must be a positive integer.";
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

export function decodeSearchParams(
  parameters: URLSearchParams,
): DecodedSearchLocation {
  const defaults = createDefaultSearchFilters();
  const locationErrors: string[] = [];
  const username = readScalar(
    parameters,
    parameterNames.username,
    defaults.username,
    locationErrors,
  );
  const maximumEffort = readScalar(
    parameters,
    parameterNames.maximumEffort,
    defaults.maximumEffort,
    locationErrors,
  );
  const filters: SearchFilters = {
    username,
    languages: readList(
      parameters,
      parameterNames.languages,
      defaults.languages,
    ),
    frameworks: readList(
      parameters,
      parameterNames.frameworks,
      defaults.frameworks,
    ),
    labels: readList(parameters, parameterNames.labels, defaults.labels),
    minimumStars: readInteger(
      parameters,
      parameterNames.minimumStars,
      defaults.minimumStars,
      locationErrors,
    ),
    maximumDifficulty: readInteger(
      parameters,
      parameterNames.maximumDifficulty,
      defaults.maximumDifficulty,
      locationErrors,
    ),
    maximumEffort: maximumEffort as EffortBand | "",
    updatedWithinDays: readInteger(
      parameters,
      parameterNames.updatedWithinDays,
      defaults.updatedWithinDays,
      locationErrors,
    ),
    includeDocumentation: readBoolean(
      parameters,
      parameterNames.includeDocumentation,
      defaults.includeDocumentation,
      locationErrors,
    ),
    includeEnglish: readBoolean(
      parameters,
      parameterNames.includeEnglish,
      defaults.includeEnglish,
      locationErrors,
    ),
    excludeArchived: readBoolean(
      parameters,
      parameterNames.excludeArchived,
      defaults.excludeArchived,
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
  };
  const searchValues = parameters.getAll(parameterNames.run);
  const shouldSearch = searchValues.length === 1 && searchValues[0] === "1";
  if (
    searchValues.length > 1 ||
    (searchValues.length === 1 && searchValues[0] !== "1")
  ) {
    locationErrors.push(`"${parameterNames.run}" must be exactly "1".`);
  }

  const errors = validateSearchFilters(filters);
  if (locationErrors.length > 0) {
    errors.form = `The shared search URL is invalid: ${locationErrors.join(
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

export function encodeSearchParams(
  filters: SearchFilters,
  shouldSearch = true,
): URLSearchParams {
  const normalized = normalizeSearchFilters(filters);
  const errors = validateSearchFilters(normalized);
  if (Object.keys(errors).length > 0) {
    throw new Error("Cannot encode invalid issue search filters.");
  }

  const parameters = new URLSearchParams();
  parameters.set(parameterNames.username, normalized.username);
  appendList(parameters, parameterNames.languages, normalized.languages);
  appendList(parameters, parameterNames.frameworks, normalized.frameworks);
  appendList(parameters, parameterNames.labels, normalized.labels);
  parameters.set(
    parameterNames.minimumStars,
    normalized.minimumStars.toString(),
  );
  parameters.set(
    parameterNames.maximumDifficulty,
    normalized.maximumDifficulty.toString(),
  );
  if (normalized.maximumEffort) {
    parameters.set(parameterNames.maximumEffort, normalized.maximumEffort);
  }
  parameters.set(
    parameterNames.updatedWithinDays,
    normalized.updatedWithinDays.toString(),
  );
  parameters.set(
    parameterNames.includeDocumentation,
    normalized.includeDocumentation.toString(),
  );
  parameters.set(
    parameterNames.includeEnglish,
    normalized.includeEnglish.toString(),
  );
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

export function toIssueSearchRequest(
  filters: SearchFilters,
): IssueSearchRequest {
  const normalized = normalizeSearchFilters(filters);
  const request: IssueSearchRequest = {
    excludeArchived: normalized.excludeArchived,
    frameworks: normalized.frameworks,
    includeDocumentation: normalized.includeDocumentation,
    includeEnglish: normalized.includeEnglish,
    labels: normalized.labels,
    languages: normalized.languages,
    maximumDifficulty: normalized.maximumDifficulty,
    minimumStars: normalized.minimumStars,
    updatedWithinDays: normalized.updatedWithinDays,
    username: normalized.username,
  };
  if (normalized.maximumEffort) {
    request.maximumEffort = normalized.maximumEffort;
  }
  return request;
}

export function normalizeSearchFilters(filters: SearchFilters): SearchFilters {
  const username = validateGitHubUsername(filters.username);
  return {
    ...filters,
    frameworks: normalizeList(filters.frameworks),
    labels: normalizeList(filters.labels),
    languages: normalizeList(filters.languages),
    username: username.valid ? username.username : filters.username.trim(),
  };
}

function validateFilterValues(
  field: "frameworks" | "labels" | "languages",
  values: string[],
  errors: SearchFilterErrors,
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

function normalizeList(values: string[]): string[] {
  const normalized: string[] = [];
  const seen = new Set<string>();
  for (const rawValue of values) {
    const value = rawValue.trim();
    const key = value.toLocaleLowerCase("en");
    if (!value || seen.has(key)) {
      continue;
    }
    seen.add(key);
    normalized.push(value);
  }
  return normalized;
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

function readList(
  parameters: URLSearchParams,
  name: string,
  fallback: string[],
): string[] {
  const values = parameters.getAll(name);
  return values.length === 0 ? [...fallback] : values;
}

function appendList(
  parameters: URLSearchParams,
  name: string,
  values: string[],
) {
  for (const value of values) {
    parameters.append(name, value);
  }
}
