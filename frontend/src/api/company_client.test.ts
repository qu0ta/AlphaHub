import { afterEach, describe, expect, it, vi } from "vitest";
import { companyApi, productsApi } from "./client";
import { fromApiCompany } from "./contracts";

vi.mock("../lib/localStore", () => ({
  localUi: { getEditedCells: vi.fn(() => []), setEditedCells: vi.fn() },
}));
const row = { id: "company-id", inn: "7700000030", name: "Test", business_type: "OOO", registration_date: "2020-01-01", main_okved: "62.01" };
const token = `header.${btoa(JSON.stringify({ sub: "user-id" }))}.signature`;
const company = { ...fromApiCompany(row), representativeName: "Иван Иванов", representativePosition: "accountant" as const, representativePositionOther: "" };
afterEach(() => {
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

describe("company and products API", () => {
  it("loads products from HTTP", async () => {
    const fetcher = vi.fn().mockResolvedValue(new Response("[]"));
    vi.stubGlobal("fetch", fetcher);
    expect(await productsApi.list()).toEqual([]);
    expect(fetcher.mock.calls[0][0]).toBe("/api/v1/products");
  });

  it("reports a missing company as 404 without inventing data", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("not found", { status: 404 })));
    await expect(companyApi.lookup(row.inn, token)).rejects.toMatchObject({ status: 404 });
  });

  it("links an unlinked user via POST /company and sends the position", async () => {
    const fetcher = vi
      .fn()
      .mockResolvedValueOnce(new Response("not found", { status: 404 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ ...row, representative_position: "Главный бухгалтер" }), { status: 201 }));
    vi.stubGlobal("fetch", fetcher);
    const saved = await companyApi.save(token, company);
    expect(saved.id).toBe(row.id);
    expect(saved.representativePosition).toBe("accountant");
    expect(fetcher.mock.calls[1][0]).toBe("/api/v1/company");
    expect(JSON.parse(fetcher.mock.calls[1][1].body)).toMatchObject({ inn: row.inn, representative_position: "Главный бухгалтер" });
  });

  it("rejects linking to a different company", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ ...row, inn: "7700000016" }))));
    await expect(companyApi.save(token, company)).rejects.toMatchObject({ status: 409 });
  });

  it("only updates the position for the already linked company", async () => {
    // Новый Response на каждый вызов: тело ответа читается только один раз
    const fetcher = vi.fn().mockImplementation(async () => new Response(JSON.stringify(row)));
    vi.stubGlobal("fetch", fetcher);
    await companyApi.save(token, { ...company, representativePosition: "other", representativePositionOther: "Финансовый директор" });
    expect(fetcher).toHaveBeenCalledTimes(2);
    expect(fetcher.mock.calls[1][0]).toBe("/api/v1/company/me");
    expect(fetcher.mock.calls[1][1].method).toBe("PATCH");
    expect(JSON.parse(fetcher.mock.calls[1][1].body)).toEqual({ representative_position: "Финансовый директор" });
  });
});
