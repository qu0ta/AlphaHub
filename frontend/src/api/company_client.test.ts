import { afterEach, describe, expect, it, vi } from "vitest";
import { companyApi, productsApi } from "./client";
import { fromApiCompany } from "./contracts";
import { mockCompany } from "./mock/server";

vi.mock("./mock/server", () => ({
  MOCK_PRODUCTS: [], mockApplications: {}, mockAuth: {}, mockBank: {},
  mockCompany: { save: vi.fn(async (_id, company) => company), lookup: vi.fn() },
  userIdFromToken: vi.fn(),
}));
const row = { id: "company-id", inn: "7700000030", name: "Test", business_type: "OOO", registration_date: "2020-01-01", main_okved: "62.01" };
const token = `header.${btoa(JSON.stringify({ sub: "user-id" }))}.signature`;
afterEach(() => { vi.unstubAllGlobals(); vi.clearAllMocks(); });

describe("real API company and products", () => {
  it("loads products from HTTP by default", async () => {
    const fetcher = vi.fn().mockResolvedValue(new Response("[]"));
    vi.stubGlobal("fetch", fetcher);
    expect(await productsApi.list()).toEqual([]);
    expect(fetcher.mock.calls[0][0]).toBe("/api/v1/products");
  });
  it("does not fabricate a company on 404", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("not found", { status: 404 })));
    await expect(companyApi.lookup(row.inn, token)).rejects.toMatchObject({ status: 404 });
    expect(mockCompany.lookup).not.toHaveBeenCalled();
  });
  it("requires invitation instead of pretending that a found company is linked", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("not found", { status: 404 })));
    await expect(companyApi.save(token, fromApiCompany(row))).rejects.toMatchObject({ status: 400 });
    expect(mockCompany.save).not.toHaveBeenCalled();
  });
  it("joins on the server before caching the company", async () => {
    const fetcher = vi.fn()
      .mockResolvedValueOnce(new Response("not found", { status: 404 }))
      .mockResolvedValueOnce(new Response(JSON.stringify(row)));
    vi.stubGlobal("fetch", fetcher);
    expect((await companyApi.save(token, fromApiCompany(row), " invitation ")).id).toBe(row.id);
    expect(fetcher.mock.calls[1][0]).toBe("/api/v1/company/join");
    expect(JSON.parse(fetcher.mock.calls[1][1].body)).toEqual({ code: "invitation" });
    expect(mockCompany.save).toHaveBeenCalledOnce();
  });
  it("does not require an invitation for the already linked company", async () => {
    const fetcher = vi.fn().mockResolvedValue(new Response(JSON.stringify(row)));
    vi.stubGlobal("fetch", fetcher);
    await companyApi.save(token, fromApiCompany(row));
    expect(fetcher).toHaveBeenCalledOnce();
  });
});
