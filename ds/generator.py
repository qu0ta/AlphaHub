import random
from datetime import date
from typing import Optional

from schemas import (
    MonthlyFinancials,
    ScoringInput,
    get_last_six_months,
)


def generate_month(month: str, revenue: int, rng: random.Random) -> MonthlyFinancials:
    inflow = round(revenue * rng.uniform(0.90, 1.05))

    outflow = round(inflow * rng.uniform(0.55, 0.80))

    taxes = round(revenue * rng.uniform(0.04, 0.08))

    return MonthlyFinancials(
        period_month=month,
        revenue_kopecks=revenue,
        operating_inflow_kopecks=inflow,
        operating_outflow_kopecks=outflow,
        tax_outflow_kopecks=taxes,
    )

def generate_company_financials(
    seed: int = 42,
    reference_date: Optional[date] = None,
) -> ScoringInput:

    rng = random.Random(seed)

    if reference_date is None:
        reference_date = date.today()

    months = get_last_six_months(reference_date)

    financials = []

    base_revenue = rng.randint(
        80_000_000,
        150_000_000
    )

    for month in months:
        revenue = round(
            base_revenue * rng.uniform(0.90, 1.10)
        )

        month_data = generate_month(
            month,
            revenue,
            rng
        )

        financials.append(month_data)

    debt_payment = round(
        base_revenue * rng.uniform(0.03, 0.12)
    )

    return ScoringInput(
        inn="7701234567",
        amount_kopecks=300_000_000,
        term_months=12,
        annual_rate_bps=1990,
        application_date=reference_date,
        financials=financials,
        declared_monthly_debt_payment_kopecks=debt_payment,
    )

if __name__ == "__main__":
    company = generate_company_financials()

    print(company.model_dump_json(indent=2))