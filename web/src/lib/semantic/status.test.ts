// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { fetchSemanticStatus } from "./status";

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

function stubFetch(payload: unknown) {
  return vi.fn().mockResolvedValue(
    new Response(JSON.stringify(payload), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    }),
  );
}

describe("fetchSemanticStatus", () => {
  it("reads the enabled/snapshot probe from /api/semantic/status", async () => {
    const fetchMock = stubFetch({ enabled: true, snapshot: false });
    vi.stubGlobal("fetch", fetchMock);
    await expect(fetchSemanticStatus()).resolves.toEqual({ enabled: true, snapshot: false });
    expect(fetchMock).toHaveBeenCalledWith("/api/semantic/status");
  });

  it("reports both capabilities independently when the branch is off", async () => {
    vi.stubGlobal("fetch", stubFetch({ enabled: false, snapshot: true }));
    await expect(fetchSemanticStatus()).resolves.toEqual({ enabled: false, snapshot: true });
  });
});
