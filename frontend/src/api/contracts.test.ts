import { describe, expect, it } from "vitest";
import { fromApiCompany, fromApiFinancials, toApiFinancials } from "./contracts";

describe("API contract adapters", () => {
  it("maps backend company to frontend company", () => {
    expect(
      fromApiCompany({
        id: "company-1",
        inn: "6671123456",
        name: "ООО Альфа",
        business_type: "ООО",
        registration_date: "2020-01-02",
        main_okved: "62.01",
      }),
    ).toMatchObject({ id: "company-1", businessType: "LLC", source: "EGRUL", okvedCode: "62.01" });
  });

  it("converts financial rubles to kopecks and back", () => {
    const rows = Array.from({ length: 6 }, (_, index) => ({
      period: `2026-${String(index + 1).padStart(2, "0")}`,
      revenue: 1000 + index,
      operatingInflow: 900,
      operatingOutflow: 500,
      taxOutflow: 100,
    }));
    const body = toApiFinancials(3, 2500, rows);
    expect(body.financials[0].revenue_kopecks).toBe(100000);
    const restored = fromApiFinancials({
      application_id: "app-1",
      data_version: 4,
      declared_monthly_debt_payment_kopecks: body.declared_monthly_debt_payment_kopecks,
      financials: body.financials,
      confirmed_at: null,
      confirmed_by_user_id: null,
    });
    expect(restored.debtPayment).toBe(2500);
    expect(restored.financials[0].revenue).toBe(1000);
  });
});
