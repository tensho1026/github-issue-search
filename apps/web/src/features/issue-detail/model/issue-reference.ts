import { appRoutes } from "../../../shared/config/app-config";
import { validateGitHubUsername } from "../../../shared/lib/github-username";
import { maximumReturnLocationLength } from "../../../shared/lib/issue-detail-location";

const repositoryPattern = /^(?!\.{1,2}$)[A-Za-z0-9._-]{1,100}$/;
const maximumSkillsPerGroup = 10;
const maximumSkillCharacters = 64;

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

  const searchValues = parsed.searchParams.getAll("search");
  const usernameValues = parsed.searchParams.getAll("username");
  const username = validateGitHubUsername(usernameValues[0] ?? "");
  const skills = readSkills(parsed.searchParams);
  if (
    searchValues.length !== 1 ||
    searchValues[0] !== "1" ||
    usernameValues.length !== 1 ||
    !username.valid ||
    skills === undefined
  ) {
    return invalidContext();
  }

  return {
    returnTo: from,
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

function readSkills(parameters: URLSearchParams): string[] | undefined {
  const languages = parameters.getAll("language");
  const frameworks = parameters.getAll("framework");
  if (
    languages.length > maximumSkillsPerGroup ||
    frameworks.length > maximumSkillsPerGroup
  ) {
    return undefined;
  }

  const result: string[] = [];
  const seen = new Set<string>();
  for (const value of [...languages, ...frameworks]) {
    if (
      value.length < 1 ||
      [...value].length > maximumSkillCharacters ||
      new TextEncoder().encode(value).byteLength > maximumSkillCharacters ||
      value.trim() !== value ||
      [...value].some((character) => {
        const codePoint = character.codePointAt(0) ?? 0;
        return (
          character === '"' ||
          character === "\\" ||
          codePoint <= 31 ||
          codePoint === 127
        );
      })
    ) {
      return undefined;
    }
    const key = value.toLocaleLowerCase("en");
    if (!seen.has(key)) {
      seen.add(key);
      result.push(value);
    }
  }
  return result;
}
