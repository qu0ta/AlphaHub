import { afterEach, describe, expect, it, vi } from "vitest";
import { request } from "./http";

afterEach(() => vi.unstubAllGlobals());

describe("request", () => {
  it("lets browser set multipart boundary for FormData", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("{}", { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    await request("/upload", { method: "POST", body: new FormData() });
    const headers = (fetchMock.mock.calls[0][1] as RequestInit).headers as Headers;
    expect(headers.has("Content-Type")).toBe(false);
  });

  it("sets JSON content type for string body", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("{}", { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    await request("/value", { method: "POST", body: JSON.stringify({ ok: true }) });
    const headers = (fetchMock.mock.calls[0][1] as RequestInit).headers as Headers;
    expect(headers.get("Content-Type")).toBe("application/json");
  });
});
