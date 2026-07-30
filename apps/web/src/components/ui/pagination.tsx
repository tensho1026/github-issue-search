import { ChevronLeft, ChevronRight } from "lucide-react";

import { Button } from "./button";
import { Icon } from "./icon";

type PaginationProps = {
  ariaLabel?: string;
  disabled?: boolean;
  hasNext: boolean;
  onPageChange: (page: number) => void;
  page: number;
  totalPages: number;
};

export function Pagination({
  ariaLabel = "Issue search pagination",
  disabled,
  hasNext,
  onPageChange,
  page,
  totalPages,
}: PaginationProps) {
  if (totalPages < 1) {
    return null;
  }
  return (
    <nav
      aria-label={ariaLabel}
      className="flex flex-wrap items-center justify-between gap-3"
    >
      <Button
        aria-label={`Go to page ${Math.max(1, page - 1)}`}
        disabled={disabled || page <= 1}
        onClick={() => onPageChange(page - 1)}
        variant="outline"
      >
        <Icon icon={ChevronLeft} />
        Previous
      </Button>
      <p aria-live="polite" className="text-sm text-muted-foreground">
        Page <strong className="text-foreground">{page}</strong> of{" "}
        <strong className="text-foreground">{totalPages}</strong>
      </p>
      <Button
        aria-label={`Go to page ${page + 1}`}
        disabled={disabled || !hasNext}
        onClick={() => onPageChange(page + 1)}
        variant="outline"
      >
        Next
        <Icon icon={ChevronRight} />
      </Button>
    </nav>
  );
}
