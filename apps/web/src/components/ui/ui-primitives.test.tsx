import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it } from "vitest";

import { Button } from "./button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
  DialogTrigger,
} from "./dialog";
import { Popover, PopoverContent, PopoverTrigger } from "./popover";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "./select";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "./tooltip";

describe("accessible UI primitives", () => {
  it("traps focus in a dialog and restores it after Escape", async () => {
    const user = userEvent.setup();
    render(
      <Dialog>
        <DialogTrigger asChild>
          <Button>Open details</Button>
        </DialogTrigger>
        <DialogContent>
          <DialogTitle>Evidence details</DialogTitle>
          <DialogDescription>Public repository signals only.</DialogDescription>
          <Button>Focusable action</Button>
        </DialogContent>
      </Dialog>,
    );

    const trigger = screen.getByRole("button", { name: "Open details" });
    await user.click(trigger);

    expect(
      screen.getByRole("dialog", { name: "Evidence details" }),
    ).toBeInTheDocument();
    expect(
      screen
        .getByRole("dialog", { name: "Evidence details" })
        .contains(document.activeElement),
    ).toBe(true);

    await user.keyboard("{Escape}");

    expect(
      screen.queryByRole("dialog", { name: "Evidence details" }),
    ).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
  });

  it("opens a popover from the keyboard and returns focus on Escape", async () => {
    const user = userEvent.setup();
    render(
      <Popover>
        <PopoverTrigger asChild>
          <Button>Privacy details</Button>
        </PopoverTrigger>
        <PopoverContent>
          Anonymous requests never use the database.
        </PopoverContent>
      </Popover>,
    );

    const trigger = screen.getByRole("button", { name: "Privacy details" });
    trigger.focus();
    await user.keyboard("{Enter}");

    expect(
      screen.getByText("Anonymous requests never use the database."),
    ).toBeVisible();

    await user.keyboard("{Escape}");

    expect(
      screen.queryByText("Anonymous requests never use the database."),
    ).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
  });

  it("changes a select value with arrow keys", async () => {
    const user = userEvent.setup();

    function ControlledSelect() {
      const [value, setValue] = useState("usage");
      return (
        <Select onValueChange={setValue} value={value}>
          <SelectTrigger aria-label="Sort technologies">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="usage">Most used</SelectItem>
            <SelectItem value="alphabetical">A–Z</SelectItem>
          </SelectContent>
        </Select>
      );
    }

    render(<ControlledSelect />);

    const trigger = screen.getByRole("combobox", {
      name: "Sort technologies",
    });
    trigger.focus();
    await user.keyboard("{ArrowDown}{ArrowDown}{Enter}");

    expect(trigger).toHaveTextContent("A–Z");
    expect(trigger).toHaveFocus();
  });

  it("exposes tooltip content to assistive technology on focus", async () => {
    const user = userEvent.setup();
    render(
      <TooltipProvider delayDuration={0}>
        <Tooltip>
          <TooltipTrigger asChild>
            <Button>Score help</Button>
          </TooltipTrigger>
          <TooltipContent>Scores are evidence weighted.</TooltipContent>
        </Tooltip>
      </TooltipProvider>,
    );

    await user.tab();

    expect(screen.getByRole("button", { name: "Score help" })).toHaveFocus();
    expect(
      await screen.findByRole("tooltip", {
        name: "Scores are evidence weighted.",
      }),
    ).toBeVisible();
  });
});
