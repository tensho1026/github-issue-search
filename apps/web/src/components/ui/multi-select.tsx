import { Check, ChevronDown, Search, X } from "lucide-react";
import { useMemo, useState } from "react";

import { cn } from "../../shared/lib/cn";
import { Button } from "./button";
import { Checkbox } from "./checkbox";
import { Icon } from "./icon";
import { Input } from "./input";
import { Popover, PopoverContent, PopoverTrigger } from "./popover";

export type MultiSelectOption = Readonly<{
  label: string;
  value: string;
}>;

type MultiSelectProps = {
  "aria-describedby"?: string;
  "aria-invalid"?: boolean;
  disabled?: boolean;
  id: string;
  maximumSelected?: number;
  onValuesChange: (values: string[]) => void;
  options: readonly MultiSelectOption[];
  placeholder: string;
  searchLabel: string;
  values: string[];
};

export function MultiSelect({
  "aria-describedby": ariaDescribedBy,
  "aria-invalid": ariaInvalid,
  disabled,
  id,
  maximumSelected = 10,
  onValuesChange,
  options,
  placeholder,
  searchLabel,
  values,
}: MultiSelectProps) {
  const [query, setQuery] = useState("");
  const allOptions = useMemo(() => {
    const optionValues = new Set(
      options.map((option) => option.value.toLocaleLowerCase("en")),
    );
    const customOptions = values
      .filter((value) => !optionValues.has(value.toLocaleLowerCase("en")))
      .map((value) => ({ label: value, value }));
    return [...options, ...customOptions];
  }, [options, values]);
  const visibleOptions = useMemo(() => {
    const normalizedQuery = query.trim().toLocaleLowerCase("en");
    return normalizedQuery
      ? allOptions.filter((option) =>
          option.label.toLocaleLowerCase("en").includes(normalizedQuery),
        )
      : allOptions;
  }, [allOptions, query]);
  const selected = new Set(
    values.map((value) => value.toLocaleLowerCase("en")),
  );

  function toggle(value: string) {
    const key = value.toLocaleLowerCase("en");
    if (selected.has(key)) {
      onValuesChange(
        values.filter(
          (selectedValue) => selectedValue.toLocaleLowerCase("en") !== key,
        ),
      );
      return;
    }
    if (values.length < maximumSelected) {
      onValuesChange([...values, value]);
    }
  }

  return (
    <Popover>
      <PopoverTrigger asChild>
        <button
          aria-describedby={ariaDescribedBy}
          aria-haspopup="dialog"
          aria-invalid={ariaInvalid}
          className={cn(
            "inline-flex min-h-11 w-full items-center justify-between gap-3 rounded-xl border border-input bg-surface px-4 py-2 text-left text-sm outline-none transition-colors hover:border-accent/45 focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50",
            ariaInvalid && "border-danger",
          )}
          disabled={disabled}
          id={id}
          type="button"
        >
          <span
            className={cn(
              "truncate",
              values.length === 0 && "text-muted-foreground",
            )}
          >
            {values.length === 0
              ? placeholder
              : values.length === 1
                ? values[0]
                : `${values.length} selected`}
          </span>
          <Icon className="text-muted-foreground" icon={ChevronDown} />
        </button>
      </PopoverTrigger>
      <PopoverContent
        align="start"
        aria-label={searchLabel}
        className="w-[min(22rem,calc(100vw-2rem))] p-3"
      >
        <div className="relative">
          <span
            aria-hidden="true"
            className="pointer-events-none absolute inset-y-0 left-3 flex items-center text-muted-foreground"
          >
            <Icon icon={Search} />
          </span>
          <Input
            aria-label={searchLabel}
            autoComplete="off"
            className="min-h-10 pl-10"
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Filter options"
            type="search"
            value={query}
          />
        </div>
        <div
          aria-label={`${searchLabel} options`}
          className="mt-3 max-h-64 overflow-y-auto overscroll-contain"
          role="group"
        >
          {visibleOptions.length > 0 ? (
            visibleOptions.map((option) => {
              const checked = selected.has(
                option.value.toLocaleLowerCase("en"),
              );
              const atLimit = values.length >= maximumSelected && !checked;
              return (
                <label
                  className={cn(
                    "flex min-h-10 cursor-pointer items-center gap-3 rounded-lg px-2 py-2 text-sm hover:bg-muted",
                    atLimit && "cursor-not-allowed opacity-45",
                  )}
                  key={option.value}
                >
                  <Checkbox
                    checked={checked}
                    disabled={atLimit}
                    onChange={() => toggle(option.value)}
                  />
                  <span className="min-w-0 flex-1 truncate">
                    {option.label}
                  </span>
                  {checked ? (
                    <Icon className="text-accent" icon={Check} />
                  ) : null}
                </label>
              );
            })
          ) : (
            <p className="px-2 py-5 text-center text-sm text-muted-foreground">
              No matching options.
            </p>
          )}
        </div>
        <div className="mt-3 flex items-center justify-between gap-3 border-t border-border pt-3">
          <span className="text-xs text-muted-foreground">
            {values.length}/{maximumSelected} selected
          </span>
          {values.length > 0 ? (
            <Button
              onClick={() => onValuesChange([])}
              size="small"
              variant="ghost"
            >
              <Icon icon={X} />
              Clear
            </Button>
          ) : null}
        </div>
      </PopoverContent>
    </Popover>
  );
}
