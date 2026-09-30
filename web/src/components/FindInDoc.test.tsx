// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { FindInDoc } from "./FindInDoc";
import {
  closeFind,
  openFind,
  resetFindOptions,
  setFindQuery,
  useFindInDoc,
} from "../lib/findInDocStore";

function Harness() {
  const [artEl, setArtEl] = useState<HTMLDivElement | null>(null);
  const { open } = useFindInDoc();
  return (
    <div ref={setArtEl}>
      {open && <FindInDoc root={artEl} mode="render" />}
      <div className="doc-rendered">
        <p>tokens and tokens</p>
      </div>
    </div>
  );
}

beforeEach(() => {
  closeFind();
  setFindQuery("");
  resetFindOptions();
});

afterEach(() => {
  closeFind();
  setFindQuery("");
  resetFindOptions();
  cleanup();
});

describe("FindInDoc", () => {
  it("applies highlights as the query is typed", async () => {
    render(<Harness />);
    act(() => openFind());
    const input = await screen.findByRole("textbox", { name: "Find in document" });
    const target = document.querySelector(".doc-rendered");
    await waitFor(() => expect(target!.querySelectorAll("mark.find-hit")).toHaveLength(0));

    await userEvent.type(input, "tokens");
    await waitFor(() => {
      expect(target!.querySelectorAll("mark.find-hit")).toHaveLength(2);
      expect(screen.getByTestId("find-count").textContent).toBe("1/2");
    });
  });

  it("steps through matches with Enter and Shift+Enter", async () => {
    render(<Harness />);
    act(() => openFind());
    const input = await screen.findByRole("textbox", { name: "Find in document" });
    await userEvent.type(input, "tokens");
    await waitFor(() => expect(screen.getByTestId("find-count").textContent).toBe("1/2"));

    await userEvent.keyboard("{enter}");
    expect(screen.getByTestId("find-count").textContent).toBe("2/2");
    const current2 = document.querySelector("mark.find-hit.is-current");
    expect(current2?.textContent).toBe("tokens");
    expect((document.querySelectorAll("mark.find-hit")[1] as HTMLElement).contains(current2)).toBe(
      true,
    );

    await userEvent.keyboard("{shift>}{enter}{/shift}");
    expect(screen.getByTestId("find-count").textContent).toBe("1/2");
  });

  it("closes on Esc", async () => {
    render(<Harness />);
    act(() => openFind());
    await screen.findByRole("textbox", { name: "Find in document" });
    await userEvent.keyboard("{escape}");
    await waitFor(() => {
      expect(screen.queryByRole("textbox", { name: "Find in document" })).toBeNull();
    });
  });

  it("exposes the match count as a live region", async () => {
    render(<Harness />);
    act(() => openFind());
    const count = await screen.findByTestId("find-count");
    expect(count.getAttribute("role")).toBe("status");
  });

  it("offers whole-word, case and regex toggles that change the match count", async () => {
    render(<Harness />);
    act(() => openFind());
    const input = await screen.findByRole("textbox", { name: "Find in document" });
    const target = document.querySelector(".doc-rendered")!;

    await userEvent.type(input, "tokens");
    await waitFor(() => expect(target.querySelectorAll("mark.find-hit")).toHaveLength(2));

    // whole word: "tokens and tokens" keeps both, "token" would too; use the
    // tokenizer case instead by narrowing with case sensitivity
    await userEvent.click(screen.getByRole("button", { name: "Match case" }));
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Match case" }).getAttribute("aria-pressed")).toBe(
        "true",
      ),
    );

    await userEvent.clear(input);
    await userEvent.type(input, "TOKENS");
    await waitFor(() => expect(screen.getByTestId("find-count").textContent).toBe("0/0"));
    await userEvent.click(screen.getByRole("button", { name: "Match case" }));
    await waitFor(() => expect(target.querySelectorAll("mark.find-hit")).toHaveLength(2));

    await userEvent.click(screen.getByRole("button", { name: "Regular expression" }));
    await userEvent.clear(input);
    await userEvent.type(input, "t\\w+ens");
    await waitFor(() => expect(target.querySelectorAll("mark.find-hit")).toHaveLength(2));
  });

  it("narrows to whole words", async () => {
    render(<Harness />);
    act(() => openFind());
    const input = await screen.findByRole("textbox", { name: "Find in document" });
    const target = document.querySelector(".doc-rendered")!;
    await userEvent.type(input, "oken");
    await waitFor(() => expect(target.querySelectorAll("mark.find-hit")).toHaveLength(2));
    await userEvent.click(screen.getByRole("button", { name: "Match whole word" }));
    await waitFor(() => expect(target.querySelectorAll("mark.find-hit")).toHaveLength(0));
  });
});
