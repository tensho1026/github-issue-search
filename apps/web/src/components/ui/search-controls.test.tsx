import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { MultiSelect } from "./multi-select";
import { Pagination } from "./pagination";

function MultiSelectHarness() {
  const [values, setValues] = useState<string[]>([]);
  return (
    <MultiSelect
      id="languages"
      onValuesChange={setValues}
      options={[
        { label: "Go", value: "Go" },
        { label: "TypeScript", value: "TypeScript" },
      ]}
      placeholder="Any language"
      searchLabel="Search languages"
      values={values}
    />
  );
}

describe("search control primitives", () => {
  it("selects, filters, clears, and restores focus in the multi-select", async () => {
    const user = userEvent.setup();
    render(<MultiSelectHarness />);
    const trigger = screen.getByRole("button", { name: "Any language" });

    await user.click(trigger);
    await user.type(
      screen.getByRole("searchbox", { name: "Search languages" }),
      "type",
    );
    await user.click(screen.getByRole("checkbox", { name: "TypeScript" }));
    expect(trigger).toHaveTextContent("TypeScript");
    await user.keyboard("{Escape}");
    expect(trigger).toHaveFocus();

    await user.click(trigger);
    await user.click(screen.getByRole("button", { name: "Clear" }));
    expect(trigger).toHaveTextContent("Any language");
  });

  it("uses server metadata to bound pagination", async () => {
    const user = userEvent.setup();
    const onPageChange = vi.fn();
    render(
      <Pagination
        hasNext
        onPageChange={onPageChange}
        page={2}
        totalPages={4}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Go to page 1" }));
    await user.click(screen.getByRole("button", { name: "Go to page 3" }));

    expect(onPageChange).toHaveBeenNthCalledWith(1, 1);
    expect(onPageChange).toHaveBeenNthCalledWith(2, 3);
    expect(screen.getByText(/page/i)).toHaveTextContent("Page 2 of 4");
  });
});
