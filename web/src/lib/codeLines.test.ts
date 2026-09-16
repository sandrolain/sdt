// @vitest-environment node
import { describe, expect, it } from "vitest";
import { lineNumbers } from "./codeLines";

describe("lineNumbers", () => {
  it("emits one number per source line", () => {
    expect(lineNumbers("a\nb\nc")).toBe("1\n2\n3");
    expect(lineNumbers("single")).toBe("1");
    expect(lineNumbers("trailing\n")).toBe("1\n2");
  });
});
