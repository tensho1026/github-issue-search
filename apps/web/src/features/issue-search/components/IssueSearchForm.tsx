import { RotateCcw, Search } from "lucide-react";
import { useEffect } from "react";
import {
  Controller,
  useForm,
  useWatch,
  type FieldErrors,
} from "react-hook-form";

import { Alert, AlertDescription } from "../../../components/ui/alert";
import { Button } from "../../../components/ui/button";
import { Checkbox } from "../../../components/ui/checkbox";
import { Field } from "../../../components/ui/field";
import { fieldDescribedBy } from "../../../components/ui/field-utils";
import { Icon } from "../../../components/ui/icon";
import { Input } from "../../../components/ui/input";
import { MultiSelect } from "../../../components/ui/multi-select";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../../../components/ui/select";
import { Slider } from "../../../components/ui/slider";
import { validateGitHubUsername } from "../../../shared/lib/github-username";
import {
  createDefaultSearchFilters,
  normalizeSearchFilters,
  searchFilterDescriptions,
  searchFilterOptions,
  validateSearchFilters,
  type SearchFilterErrors,
  type SearchFilters,
} from "../model/search-filters";

type IssueSearchFormProps = {
  defaultValues: SearchFilters;
  disabled?: boolean;
  locationErrors?: SearchFilterErrors;
  onSubmit: (filters: SearchFilters) => void;
};

type ToggleProps = {
  checked: boolean;
  description: string;
  id: string;
  label: string;
  onChange: (checked: boolean) => void;
};

function Toggle({ checked, description, id, label, onChange }: ToggleProps) {
  return (
    <label
      className="flex cursor-pointer items-start gap-3 rounded-xl border border-border bg-muted/40 p-4 transition-colors hover:border-accent/35 hover:bg-muted"
      htmlFor={id}
    >
      <Checkbox
        checked={checked}
        id={id}
        onChange={(event) => onChange(event.target.checked)}
      />
      <span>
        <span className="block text-sm font-semibold">{label}</span>
        <span className="mt-1 block text-xs leading-5 text-muted-foreground">
          {description}
        </span>
      </span>
    </label>
  );
}

function messageFor(
  errors: FieldErrors<SearchFilters>,
  field: keyof SearchFilters,
): string | undefined {
  const message = errors[field]?.message;
  return typeof message === "string" ? message : undefined;
}

export function IssueSearchForm({
  defaultValues,
  disabled,
  locationErrors,
  onSubmit,
}: IssueSearchFormProps) {
  const {
    control,
    formState: { errors },
    handleSubmit,
    register,
    reset,
    setError,
  } = useForm<SearchFilters>({
    defaultValues,
    mode: "onSubmit",
    reValidateMode: "onChange",
  });
  useEffect(() => {
    reset(defaultValues);
  }, [defaultValues, reset]);
  const difficulty =
    useWatch({ control, name: "maximumDifficulty" }) ??
    defaultValues.maximumDifficulty;
  const difficultyLabel =
    searchFilterOptions.difficulties.find(
      (option) => option.value === difficulty,
    )?.label ?? `Level ${difficulty}`;

  const submit = handleSubmit((values) => {
    const normalized = normalizeSearchFilters({ ...values, page: 1 });
    const validationErrors = validateSearchFilters(normalized);
    const entries = Object.entries(validationErrors) as Array<
      [keyof SearchFilters | "form", string]
    >;
    if (entries.length > 0) {
      for (const [field, message] of entries) {
        if (field !== "form") {
          setError(field, { message, type: "validate" });
        }
      }
      return;
    }
    onSubmit(normalized);
  });

  const usernameError =
    messageFor(errors, "username") ?? locationErrors?.username;
  const languagesError =
    messageFor(errors, "languages") ?? locationErrors?.languages;
  const frameworksError =
    messageFor(errors, "frameworks") ?? locationErrors?.frameworks;
  const labelsError = messageFor(errors, "labels") ?? locationErrors?.labels;
  const minimumStarsError =
    messageFor(errors, "minimumStars") ?? locationErrors?.minimumStars;
  const difficultyError =
    messageFor(errors, "maximumDifficulty") ??
    locationErrors?.maximumDifficulty;
  const effortError =
    messageFor(errors, "maximumEffort") ?? locationErrors?.maximumEffort;
  const recencyError =
    messageFor(errors, "updatedWithinDays") ??
    locationErrors?.updatedWithinDays;
  const pageSizeError =
    messageFor(errors, "perPage") ?? locationErrors?.perPage;

  return (
    <form
      className="grid gap-6"
      noValidate
      onSubmit={(event) => {
        void submit(event);
      }}
    >
      {locationErrors?.form ? (
        <Alert variant="danger">
          <AlertDescription>{locationErrors.form}</AlertDescription>
        </Alert>
      ) : null}

      <Field
        description="Recommendations are matched against this public GitHub profile."
        error={usernameError}
        htmlFor="search-username"
        label="GitHub username"
      >
        <Input
          aria-describedby={fieldDescribedBy(
            "search-username",
            true,
            Boolean(usernameError),
          )}
          aria-invalid={Boolean(usernameError)}
          autoCapitalize="none"
          autoComplete="username"
          id="search-username"
          placeholder="octocat"
          spellCheck={false}
          {...register("username", {
            validate(value) {
              const result = validateGitHubUsername(value);
              return result.valid ? true : result.message;
            },
          })}
        />
      </Field>

      <div className="grid gap-5 xl:grid-cols-2">
        <Controller
          control={control}
          name="languages"
          render={({ field }) => (
            <Field
              description={searchFilterDescriptions.languages}
              error={languagesError}
              htmlFor="search-languages"
              label="Languages"
            >
              <MultiSelect
                aria-describedby={fieldDescribedBy(
                  "search-languages",
                  true,
                  Boolean(languagesError),
                )}
                aria-invalid={Boolean(languagesError)}
                id="search-languages"
                onValuesChange={field.onChange}
                options={searchFilterOptions.languages}
                placeholder="Any primary language"
                searchLabel="Search languages"
                values={field.value}
              />
            </Field>
          )}
        />
        <Controller
          control={control}
          name="frameworks"
          render={({ field }) => (
            <Field
              description={searchFilterDescriptions.frameworks}
              error={frameworksError}
              htmlFor="search-frameworks"
              label="Frameworks"
            >
              <MultiSelect
                aria-describedby={fieldDescribedBy(
                  "search-frameworks",
                  true,
                  Boolean(frameworksError),
                )}
                aria-invalid={Boolean(frameworksError)}
                id="search-frameworks"
                onValuesChange={field.onChange}
                options={searchFilterOptions.frameworks}
                placeholder="Any framework"
                searchLabel="Search frameworks"
                values={field.value}
              />
            </Field>
          )}
        />
      </div>

      <Controller
        control={control}
        name="labels"
        render={({ field }) => (
          <Field
            description={searchFilterDescriptions.labels}
            error={labelsError}
            htmlFor="search-labels"
            label="Issue labels"
          >
            <MultiSelect
              aria-describedby={fieldDescribedBy(
                "search-labels",
                true,
                Boolean(labelsError),
              )}
              aria-invalid={Boolean(labelsError)}
              id="search-labels"
              onValuesChange={field.onChange}
              options={searchFilterOptions.labels}
              placeholder="Default starter labels"
              searchLabel="Search issue labels"
              values={field.value}
            />
          </Field>
        )}
      />

      <div className="grid gap-5 sm:grid-cols-2">
        <Field
          description="Repositories below this star count are excluded."
          error={minimumStarsError}
          htmlFor="search-minimum-stars"
          label="Minimum repository stars"
        >
          <Input
            aria-describedby={fieldDescribedBy(
              "search-minimum-stars",
              true,
              Boolean(minimumStarsError),
            )}
            aria-invalid={Boolean(minimumStarsError)}
            id="search-minimum-stars"
            inputMode="numeric"
            min={0}
            type="number"
            {...register("minimumStars", {
              min: {
                message: "Minimum stars cannot be negative.",
                value: 0,
              },
              valueAsNumber: true,
            })}
          />
        </Field>
        <Field
          description="Both the issue and repository must be this recent."
          error={recencyError}
          htmlFor="search-recency"
          label="Updated within days"
        >
          <Input
            aria-describedby={fieldDescribedBy(
              "search-recency",
              true,
              Boolean(recencyError),
            )}
            aria-invalid={Boolean(recencyError)}
            id="search-recency"
            inputMode="numeric"
            max={3650}
            min={1}
            type="number"
            {...register("updatedWithinDays", {
              max: {
                message: "Recency cannot exceed 3650 days.",
                value: 3650,
              },
              min: {
                message: "Recency must be at least one day.",
                value: 1,
              },
              valueAsNumber: true,
            })}
          />
        </Field>
      </div>

      <div className="grid gap-5 sm:grid-cols-2">
        <Field
          description={`Current maximum: ${difficultyLabel}.`}
          error={difficultyError}
          htmlFor="search-difficulty"
          label="Maximum difficulty"
        >
          <Slider
            aria-describedby={fieldDescribedBy(
              "search-difficulty",
              true,
              Boolean(difficultyError),
            )}
            aria-invalid={Boolean(difficultyError)}
            aria-valuetext={difficultyLabel}
            id="search-difficulty"
            max={5}
            min={1}
            step={1}
            {...register("maximumDifficulty", {
              max: 5,
              min: 1,
              valueAsNumber: true,
            })}
          />
        </Field>
        <Controller
          control={control}
          name="maximumEffort"
          render={({ field }) => (
            <Field
              description="Applied after full issue analysis and before pagination."
              error={effortError}
              htmlFor="search-effort"
              label="Available time"
            >
              <Select
                onValueChange={(value) =>
                  field.onChange(value === "any" ? "" : value)
                }
                value={field.value || "any"}
              >
                <SelectTrigger
                  aria-describedby={fieldDescribedBy(
                    "search-effort",
                    true,
                    Boolean(effortError),
                  )}
                  aria-invalid={Boolean(effortError)}
                  className="w-full rounded-xl"
                  id="search-effort"
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {searchFilterOptions.efforts.map((option) => (
                    <SelectItem
                      key={option.value || "any"}
                      value={option.value || "any"}
                    >
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>
          )}
        />
      </div>

      <fieldset className="grid gap-3">
        <legend className="mb-2 text-sm font-semibold">Eligibility</legend>
        <div className="grid gap-3 xl:grid-cols-3">
          <Controller
            control={control}
            name="includeDocumentation"
            render={({ field }) => (
              <Toggle
                checked={field.value}
                description="Add documentation-labelled issues."
                id="search-documentation"
                label="Include documentation"
                onChange={field.onChange}
              />
            )}
          />
          <Controller
            control={control}
            name="includeEnglish"
            render={({ field }) => (
              <Toggle
                checked={field.value}
                description="Allow predominantly Latin-script issue text."
                id="search-english"
                label="Include English issues"
                onChange={field.onChange}
              />
            )}
          />
          <Controller
            control={control}
            name="excludeArchived"
            render={({ field }) => (
              <Toggle
                checked={field.value}
                description="Hide repositories that no longer accept changes."
                id="search-archived"
                label="Exclude archived repositories"
                onChange={field.onChange}
              />
            )}
          />
        </div>
      </fieldset>

      <Controller
        control={control}
        name="perPage"
        render={({ field }) => (
          <Field
            className="max-w-xs"
            description="The server remains the pagination source of truth."
            error={pageSizeError}
            htmlFor="search-page-size"
            label="Results per page"
          >
            <Select
              onValueChange={(value) => field.onChange(Number(value))}
              value={field.value.toString()}
            >
              <SelectTrigger
                aria-describedby={fieldDescribedBy(
                  "search-page-size",
                  true,
                  Boolean(pageSizeError),
                )}
                aria-invalid={Boolean(pageSizeError)}
                className="w-full rounded-xl"
                id="search-page-size"
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {searchFilterOptions.pageSizes.map((option) => (
                  <SelectItem
                    key={option.value}
                    value={option.value.toString()}
                  >
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
        )}
      />

      <div className="flex flex-wrap gap-3 border-t border-border pt-5">
        <Button disabled={disabled} type="submit">
          <Icon icon={Search} />
          {disabled ? "Searching…" : "Find ranked issues"}
        </Button>
        <Button
          disabled={disabled}
          onClick={() =>
            reset(createDefaultSearchFilters(defaultValues.username))
          }
          type="button"
          variant="ghost"
        >
          <Icon icon={RotateCcw} />
          Reset filters
        </Button>
      </div>
    </form>
  );
}
