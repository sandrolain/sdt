import { describe, expect, it } from "vitest";
import { mermaidFencePlugin } from "./slidesMermaid";

interface FenceToken {
  info: string;
  content: string;
}

type Rule = (
  tokens: FenceToken[],
  idx: number,
  options: unknown,
  env: unknown,
  self: unknown,
) => string;

function fakeMarkdownIt(fallback = "FALLBACK"): {
  md: { renderer: { rules: { fence?: Rule } } };
  calls: unknown[][];
} {
  const calls: unknown[][] = [];
  const md = {
    renderer: {
      rules: {
        fence: (...args: unknown[]) => {
          calls.push(args);
          return fallback;
        },
      } as { fence?: Rule },
    },
  };
  return { md, calls };
}

function token(info: string, content = "flowchart LR\n  A-->B"): FenceToken[] {
  return [{ info, content }];
}

describe("mermaidFencePlugin", () => {
  it("turns a mermaid fence into the viewer placeholder", () => {
    const { md, calls } = fakeMarkdownIt();
    mermaidFencePlugin(md);
    const out = md.renderer.rules.fence!(token("mermaid"), 0, {}, {}, {});
    expect(out).toBe(
      `<div class="md-mermaid" data-src="${encodeURIComponent("flowchart LR\n  A-->B")}"></div>`,
    );
    expect(calls).toHaveLength(0);
  });

  it("matches a fence info string with trailing words", () => {
    const { md } = fakeMarkdownIt();
    mermaidFencePlugin(md);
    const out = md.renderer.rules.fence!(token("mermaid title"), 0, {}, {}, {});
    expect(out).toContain('class="md-mermaid"');
  });

  it("delegates every other language to the original rule", () => {
    const { md, calls } = fakeMarkdownIt("GO");
    mermaidFencePlugin(md);
    const out = md.renderer.rules.fence!(token("go"), 0, {}, {}, {});
    expect(out).toBe("GO");
    expect(calls).toHaveLength(1);
  });

  it("leaves a markdown-it without a fence rule alone", () => {
    const md = { renderer: { rules: {} } };
    expect(() => mermaidFencePlugin(md)).not.toThrow();
  });
});
