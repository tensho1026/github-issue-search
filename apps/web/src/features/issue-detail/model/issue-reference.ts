import { appRoutes } from "../../../shared/config/app-config";
import { validateGitHubUsername } from "../../../shared/lib/github-username";
import {
  decodeSearchParams,
  encodeSearchParams,
} from "../../issue-search/model/search-filters";

const repositoryPattern = /^(?!\.{1,2}$)[A-Za-z0-9._-]{1,100}$/;
const maximumReturnLocationLength = 4096;

export type IssueReference =
  | {
      issueNumber: number;
      owner: string;
      repository: string;
      valid: true;
    }
  | {
      message: string;
      valid: false;
    };

export type IssueDetailContext =
  | {
      returnTo: string;
      skills: string[];
      valid: true;
    }
  | {
      message: string;
      valid: false;
    };

export function validateIssueReference(
  ownerValue: string | undefined,
  repositoryValue: string | undefined,
  issueNumberValue: string | undefined,
): IssueReference {
  const owner = validateGitHubUsername(ownerValue ?? "");
  if (!owner.valid) {
    return {
      message: "The issue owner is not a valid GitHub login.",
      valid: false,
    };
  }

  const repository = repositoryValue?.trim() ?? "";
  if (!repositoryPattern.test(repository)) {
    return {
      message: "The repository name is not a valid GitHub repository name.",
      valid: false,
    };
  }

  if (!/^[1-9]\d*$/.test(issueNumberValue ?? "")) {
    return {
      message: "The issue number must be a positive integer.",
      valid: false,
    };
  }
  const issueNumber = Number(issueNumberValue);
  if (!Number.isSafeInteger(issueNumber)) {
    return {
      message: "The issue number is outside the supported range.",
      valid: false,
    };
  }

  return {
    issueNumber,
    owner: owner.username,
    repository,
    valid: true,
  };
}

export function issueDetailSearchParameters(
  currentLocation: string,
): URLSearchParams {
  const parameters = new URLSearchParams();
  if (
    currentLocation.length <= maximumReturnLocationLength &&
    currentLocation.startsWith(`${appRoutes.search}?`)
  ) {
    parameters.set("from", currentLocation);
  }
  return parameters;
}

export function decodeIssueDetailContext(
  parameters: URLSearchParams,
): IssueDetailContext {
  const fromValues = parameters.getAll("from");
  if (fromValues.length === 0) {
    return { returnTo: appRoutes.search, skills: [], valid: true };
  }
  if (fromValues.length !== 1) {
    return invalidContext();
  }

  const from = fromValues[0] ?? "";
  if (
    from.length > maximumReturnLocationLength ||
    !from.startsWith(`${appRoutes.search}?`)
  ) {
    return invalidContext();
  }

  const parsed = new URL(from, "https://issuescout.invalid");
  if (
    parsed.origin !== "https://issuescout.invalid" ||
    parsed.pathname !== appRoutes.search ||
    parsed.hash
  ) {
    return invalidContext();
  }

  const search = decodeSearchParams(parsed.searchParams);
  if (!search.valid || !search.shouldSearch) {
    return invalidContext();
  }

  const skills = normalizeSkills([
    ...search.filters.languages,
    ...search.filters.frameworks,
  ]);
  return {
    returnTo: `${appRoutes.search}?${encodeSearchParams(
      search.filters,
    ).toString()}`,
    skills,
    valid: true,
  };
}

function invalidContext(): IssueDetailContext {
  return {
    message:
      "The return location is invalid. Open this issue from a new search.",
    valid: false,
  };
}

function normalizeSkills(values: readonly string[]): string[] {
  const result: string[] = [];
  const seen = new Set<string>();
  for (const value of values) {
    const key = value.toLocaleLowerCase("en");
    if (!seen.has(key)) {
      seen.add(key);
      result.push(value);
    }
  }
  return result.slice(0, 20);
}
