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
import {
  createDefaultRepositoryFilters,
  normalizeRepositoryFilters,
  repositoryFilterDescriptions,
  repositoryFilterOptions,
  validateRepositoryFilters,
  type RepositoryFilterErrors,
  type RepositoryFilters,
} from "../model/repository-filters";

type RepositoryDiscoveryFormProps = {
  defaultValues: RepositoryFilters;
  disabled?: boolean;
  locationErrors?: RepositoryFilterErrors;
  onSubmit: (filters: RepositoryFilters) => void;
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
  errors: FieldErrors<RepositoryFilters>,
  field: keyof RepositoryFilters,
): string | undefined {
  const message = errors[field]?.message;
  return typeof message === "string" ? message : undefined;
}

export function RepositoryDiscoveryForm({
  defaultValues,
  disabled,
  locationErrors,
  onSubmit,
}: RepositoryDiscoveryFormProps) {
  const {
    control,
    formState: { errors },
    handleSubmit,
    register,
    reset,
    setError,
  } = useForm<RepositoryFilters>({
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
  const readiness =
    useWatch({ control, name: "minimumReadiness" }) ??
    defaultValues.minimumReadiness;
  const difficultyLabel =
    repositoryFilterOptions.difficulties.find(
      (option) => option.value === difficulty,
    )?.label ?? `Level ${difficulty}`;

  const submit = handleSubmit((values) => {
    const normalized = normalizeRepositoryFilters({ ...values, page: 1 });
    const validationErrors = validateRepositoryFilters(normalized);
    const entries = Object.entries(validationErrors) as Array<
      [keyof RepositoryFilters | "form", string]
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

  function errorFor(field: keyof RepositoryFilters): string | undefined {
    return messageFor(errors, field) ?? locationErrors?.[field];
  }

  const languagesError = errorFor("languages");
  const technologiesError = errorFor("technologies");
  const licensesError = errorFor("licenses");
  const categoriesError = errorFor("categories");
  const minimumStarsError = errorFor("minimumStars");
  const minimumForksError = errorFor("minimumForks");
  const minimumOpenIssuesError = errorFor("minimumOpenIssues");
  const maximumOpenIssuesError = errorFor("maximumOpenIssues");
  const recencyError = errorFor("updatedWithinDays");
  const difficultyError = errorFor("maximumDifficulty");
  const readinessError = errorFor("minimumReadiness");
  const japaneseReadmeError = errorFor("hasJapaneseReadme");
  const forkPolicyError = errorFor("forkPolicy");
  const pageSizeError = errorFor("perPage");

  return (
    <form
      className="grid gap-7"
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

      <fieldset className="grid gap-5">
        <legend className="mb-1 text-sm font-semibold">
          Technology and purpose
        </legend>
        <div className="grid gap-5 xl:grid-cols-2">
          <Controller
            control={control}
            name="languages"
            render={({ field }) => (
              <Field
                description={repositoryFilterDescriptions.languages}
                error={languagesError}
                htmlFor="repository-languages"
                label="Languages"
              >
                <MultiSelect
                  aria-describedby={fieldDescribedBy(
                    "repository-languages",
                    true,
                    Boolean(languagesError),
                  )}
                  aria-invalid={Boolean(languagesError)}
                  id="repository-languages"
                  onValuesChange={field.onChange}
                  options={repositoryFilterOptions.languages}
                  placeholder="Any primary language"
                  searchLabel="Search repository languages"
                  values={field.value}
                />
              </Field>
            )}
          />
          <Controller
            control={control}
            name="technologies"
            render={({ field }) => (
              <Field
                description={repositoryFilterDescriptions.technologies}
                error={technologiesError}
                htmlFor="repository-technologies"
                label="Frameworks and technologies"
              >
                <MultiSelect
                  aria-describedby={fieldDescribedBy(
                    "repository-technologies",
                    true,
                    Boolean(technologiesError),
                  )}
                  aria-invalid={Boolean(technologiesError)}
                  id="repository-technologies"
                  onValuesChange={field.onChange}
                  options={repositoryFilterOptions.technologies}
                  placeholder="Any technology"
                  searchLabel="Search repository technologies"
                  values={field.value}
                />
              </Field>
            )}
          />
        </div>
        <div className="grid gap-5 xl:grid-cols-2">
          <Controller
            control={control}
            name="licenses"
            render={({ field }) => (
              <Field
                description={repositoryFilterDescriptions.licenses}
                error={licensesError}
                htmlFor="repository-licenses"
                label="SPDX licenses"
              >
                <MultiSelect
                  aria-describedby={fieldDescribedBy(
                    "repository-licenses",
                    true,
                    Boolean(licensesError),
                  )}
                  aria-invalid={Boolean(licensesError)}
                  id="repository-licenses"
                  onValuesChange={field.onChange}
                  options={repositoryFilterOptions.licenses}
                  placeholder="Any recognized license"
                  searchLabel="Search SPDX licenses"
                  values={field.value}
                />
              </Field>
            )}
          />
          <Controller
            control={control}
            name="categories"
            render={({ field }) => (
              <Field
                description={repositoryFilterDescriptions.categories}
                error={categoriesError}
                htmlFor="repository-categories"
                label="OSS categories"
              >
                <MultiSelect
                  aria-describedby={fieldDescribedBy(
                    "repository-categories",
                    true,
                    Boolean(categoriesError),
                  )}
                  aria-invalid={Boolean(categoriesError)}
                  id="repository-categories"
                  onValuesChange={field.onChange}
                  options={repositoryFilterOptions.categories}
                  placeholder="Any category"
                  searchLabel="Search OSS categories"
                  values={field.value}
                />
              </Field>
            )}
          />
        </div>
      </fieldset>

      <fieldset className="grid gap-5">
        <legend className="mb-1 text-sm font-semibold">
          Popularity and activity
        </legend>
        <div className="grid gap-5 sm:grid-cols-2 xl:grid-cols-4">
          <Field
            description="Inclusive lower bound."
            error={minimumStarsError}
            htmlFor="repository-minimum-stars"
            label="Minimum stars"
          >
            <Input
              aria-describedby={fieldDescribedBy(
                "repository-minimum-stars",
                true,
                Boolean(minimumStarsError),
              )}
              aria-invalid={Boolean(minimumStarsError)}
              id="repository-minimum-stars"
              inputMode="numeric"
              min={0}
              type="number"
              {...register("minimumStars", { min: 0, valueAsNumber: true })}
            />
          </Field>
          <Field
            description="Inclusive lower bound."
            error={minimumForksError}
            htmlFor="repository-minimum-forks"
            label="Minimum forks"
          >
            <Input
              aria-describedby={fieldDescribedBy(
                "repository-minimum-forks",
                true,
                Boolean(minimumForksError),
              )}
              aria-invalid={Boolean(minimumForksError)}
              id="repository-minimum-forks"
              inputMode="numeric"
              min={0}
              type="number"
              {...register("minimumForks", { min: 0, valueAsNumber: true })}
            />
          </Field>
          <Field
            description="Repositories need at least this many."
            error={minimumOpenIssuesError}
            htmlFor="repository-minimum-open-issues"
            label="Minimum open issues"
          >
            <Input
              aria-describedby={fieldDescribedBy(
                "repository-minimum-open-issues",
                true,
                Boolean(minimumOpenIssuesError),
              )}
              aria-invalid={Boolean(minimumOpenIssuesError)}
              id="repository-minimum-open-issues"
              inputMode="numeric"
              min={0}
              type="number"
              {...register("minimumOpenIssues", {
                min: 0,
                valueAsNumber: true,
              })}
            />
          </Field>
          <Field
            description="Keeps the issue surface manageable."
            error={maximumOpenIssuesError}
            htmlFor="repository-maximum-open-issues"
            label="Maximum open issues"
          >
            <Input
              aria-describedby={fieldDescribedBy(
                "repository-maximum-open-issues",
                true,
                Boolean(maximumOpenIssuesError),
              )}
              aria-invalid={Boolean(maximumOpenIssuesError)}
              id="repository-maximum-open-issues"
              inputMode="numeric"
              min={0}
              type="number"
              {...register("maximumOpenIssues", {
                min: 0,
                valueAsNumber: true,
              })}
            />
          </Field>
        </div>
        <Field
          className="max-w-sm"
          description="Maximum age of the latest public push, from 1 to 3650 days."
          error={recencyError}
          htmlFor="repository-recency"
          label="Updated within days"
        >
          <Input
            aria-describedby={fieldDescribedBy(
              "repository-recency",
              true,
              Boolean(recencyError),
            )}
            aria-invalid={Boolean(recencyError)}
            id="repository-recency"
            inputMode="numeric"
            max={3650}
            min={1}
            type="number"
            {...register("updatedWithinDays", {
              max: 3650,
              min: 1,
              valueAsNumber: true,
            })}
          />
        </Field>
      </fieldset>

      <fieldset className="grid gap-5">
        <legend className="mb-1 text-sm font-semibold">
          Contribution readiness
        </legend>
        <div className="grid gap-5 xl:grid-cols-2">
          <Field
            description={`Current maximum: ${difficultyLabel}. The server explains this preliminary repository-level estimate.`}
            error={difficultyError}
            htmlFor="repository-difficulty"
            label="Maximum difficulty"
          >
            <Slider
              aria-describedby={fieldDescribedBy(
                "repository-difficulty",
                true,
                Boolean(difficultyError),
              )}
              aria-invalid={Boolean(difficultyError)}
              aria-valuetext={difficultyLabel}
              id="repository-difficulty"
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
          <Field
            description={`Current minimum: ${readiness}/100. The server combines bounded public quality signals.`}
            error={readinessError}
            htmlFor="repository-readiness"
            label="Minimum readiness"
          >
            <Slider
              aria-describedby={fieldDescribedBy(
                "repository-readiness",
                true,
                Boolean(readinessError),
              )}
              aria-invalid={Boolean(readinessError)}
              aria-valuetext={`${readiness} out of 100`}
              id="repository-readiness"
              max={100}
              min={0}
              step={5}
              {...register("minimumReadiness", {
                max: 100,
                min: 0,
                valueAsNumber: true,
              })}
            />
          </Field>
        </div>
        <div className="grid gap-5 xl:grid-cols-2">
          <Controller
            control={control}
            name="hasJapaneseReadme"
            render={({ field }) => (
              <Field
                description="Japanese detection is heuristic evidence, not guaranteed language classification."
                error={japaneseReadmeError}
                htmlFor="repository-japanese-readme"
                label="Japanese README"
              >
                <Select onValueChange={field.onChange} value={field.value}>
                  <SelectTrigger
                    aria-describedby={fieldDescribedBy(
                      "repository-japanese-readme",
                      true,
                      Boolean(japaneseReadmeError),
                    )}
                    aria-invalid={Boolean(japaneseReadmeError)}
                    className="w-full rounded-xl"
                    id="repository-japanese-readme"
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {repositoryFilterOptions.japaneseReadmes.map((option) => (
                      <SelectItem key={option.value} value={option.value}>
                        {option.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </Field>
            )}
          />
          <Controller
            control={control}
            name="forkPolicy"
            render={({ field }) => (
              <Field
                description="Choose whether GitHub forks are eligible."
                error={forkPolicyError}
                htmlFor="repository-fork-policy"
                label="Fork policy"
              >
                <Select onValueChange={field.onChange} value={field.value}>
                  <SelectTrigger
                    aria-describedby={fieldDescribedBy(
                      "repository-fork-policy",
                      true,
                      Boolean(forkPolicyError),
                    )}
                    aria-invalid={Boolean(forkPolicyError)}
                    className="w-full rounded-xl"
                    id="repository-fork-policy"
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {repositoryFilterOptions.forkPolicies.map((option) => (
                      <SelectItem key={option.value} value={option.value}>
                        {option.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </Field>
            )}
          />
        </div>
        <Controller
          control={control}
          name="excludeArchived"
          render={({ field }) => (
            <Toggle
              checked={field.value}
              description="Hide repositories that no longer accept changes."
              id="repository-exclude-archived"
              label="Exclude archived repositories"
              onChange={field.onChange}
            />
          )}
        />
      </fieldset>

      <Controller
        control={control}
        name="perPage"
        render={({ field }) => (
          <Field
            className="max-w-xs"
            description="The server remains the pagination source of truth."
            error={pageSizeError}
            htmlFor="repository-page-size"
            label="Results per page"
          >
            <Select
              onValueChange={(value) => field.onChange(Number(value))}
              value={field.value.toString()}
            >
              <SelectTrigger
                aria-describedby={fieldDescribedBy(
                  "repository-page-size",
                  true,
                  Boolean(pageSizeError),
                )}
                aria-invalid={Boolean(pageSizeError)}
                className="w-full rounded-xl"
                id="repository-page-size"
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {repositoryFilterOptions.pageSizes.map((option) => (
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
          {disabled ? "Searching…" : "Discover repositories"}
        </Button>
        <Button
          disabled={disabled}
          onClick={() => reset(createDefaultRepositoryFilters())}
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
