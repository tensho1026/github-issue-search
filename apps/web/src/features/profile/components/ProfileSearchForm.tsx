import { ArrowRight, AtSign } from "lucide-react";
import { useForm } from "react-hook-form";
import { useNavigate } from "react-router";

import { Button } from "../../../components/ui/button";
import { Field } from "../../../components/ui/field";
import { fieldDescribedBy } from "../../../components/ui/field-utils";
import { Icon } from "../../../components/ui/icon";
import { Input } from "../../../components/ui/input";
import { appRoutes } from "../../../shared/config/app-config";
import {
  gitHubUsernameLimits,
  validateGitHubUsername,
} from "../../../shared/lib/github-username";
import { cn } from "../../../shared/lib/cn";

type ProfileSearchFields = {
  username: string;
};

type ProfileSearchFormProps = {
  className?: string;
  compact?: boolean;
  defaultUsername?: string;
};

const usernameInputId = "github-username";
const usernameDescription =
  "Public profiles only. Enter the login shown in a GitHub profile URL.";

export function ProfileSearchForm({
  className,
  compact = false,
  defaultUsername = "",
}: ProfileSearchFormProps) {
  const navigate = useNavigate();
  const {
    formState: { errors, isSubmitting },
    handleSubmit,
    register,
  } = useForm<ProfileSearchFields>({
    defaultValues: { username: defaultUsername },
    mode: "onSubmit",
    reValidateMode: "onChange",
  });

  const submit = handleSubmit(({ username }) => {
    const validation = validateGitHubUsername(username);
    if (validation.valid) {
      void navigate(appRoutes.profile(validation.username));
    }
  });
  const usernameRegistration = register("username", {
    maxLength: {
      message: `GitHub usernames contain at most ${gitHubUsernameLimits.maximumLength} characters.`,
      value: gitHubUsernameLimits.maximumLength,
    },
    validate(value) {
      const validation = validateGitHubUsername(value);
      return validation.valid ? true : validation.message;
    },
  });
  const error = errors.username?.message;

  return (
    <form
      className={cn(
        "grid gap-4",
        compact
          ? "sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end"
          : "sm:grid-cols-[minmax(0,1fr)_auto] sm:items-start",
        className,
      )}
      noValidate
      onSubmit={(event) => {
        void submit(event);
      }}
    >
      <Field
        className={compact ? "gap-2" : undefined}
        description={compact ? undefined : usernameDescription}
        error={error}
        htmlFor={usernameInputId}
        label="GitHub username"
      >
        <div className="relative">
          <span
            aria-hidden="true"
            className="pointer-events-none absolute inset-y-0 left-4 flex items-center text-muted-foreground"
          >
            <Icon icon={AtSign} />
          </span>
          <Input
            aria-describedby={fieldDescribedBy(
              usernameInputId,
              !compact,
              Boolean(error),
            )}
            aria-invalid={Boolean(error)}
            autoCapitalize="none"
            autoComplete="username"
            className="pl-11"
            id={usernameInputId}
            inputMode="text"
            maxLength={gitHubUsernameLimits.maximumLength + 1}
            placeholder="octocat"
            spellCheck={false}
            {...usernameRegistration}
          />
        </div>
      </Field>
      <Button
        className={cn(!compact && "mt-[1.625rem]")}
        disabled={isSubmitting}
        size={compact ? "default" : "large"}
        type="submit"
      >
        Analyze profile
        <Icon icon={ArrowRight} />
      </Button>
    </form>
  );
}
