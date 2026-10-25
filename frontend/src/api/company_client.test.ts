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
  it("links an unlinked user via POST /company (backend creates or joins by INN)", async () => {
    const fetcher = vi.fn()
      .mockResolvedValueOnce(new Response("not found", { status: 404 }))
      .mockResolvedValueOnce(new Response(JSON.stringify(row), { status: 201 }));
    vi.stubGlobal("fetch", fetcher);
    expect((await companyApi.save(token, fromApiCompany(row))).id).toBe(row.id);
    expect(fetcher.mock.calls[1][0]).toBe("/api/v1/company");
    expect(JSON.parse(fetcher.mock.calls[1][1].body)).toMatchObject({ inn: row.inn });
    expect(mockCompany.save).toHaveBeenCalledOnce();
  });
  it("rejects linking to a different company", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ ...row, inn: "7700000016" }))));
    await expect(companyApi.save(token, fromApiCompany(row))).rejects.toMatchObject({ status: 409 });
    expect(mockCompany.save).not.toHaveBeenCalled();
  });
  it("does not post again for the already linked company", async () => {
    const fetcher = vi.fn().mockResolvedValue(new Response(JSON.stringify(row)));
    vi.stubGlobal("fetch", fetcher);
    await companyApi.save(token, fromApiCompany(row));
    expect(fetcher).toHaveBeenCalledOnce();
  });
});
