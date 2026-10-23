import random
from datetime import date
from typing import Optional

from schemas import (
    MonthlyFinancials,
    ScoringInput,
    get_last_six_months,
)


PROFILES = {
    "stable": {
        "inn": "7700000016",
        "revenue_volatility": 0.04,
        "monthly_trend": 0.0,
        "inflow_ratio": (0.95, 1.05),
        "outflow_ratio": (0.50, 0.68),
        "tax_ratio": (0.04, 0.07),
        "debt_ratio": (0.02, 0.06),
        "amount_kopecks": 300_000_000,
        "term_months": 12,
    },
    "medium": {
        "inn": "7700000023",
        "revenue_volatility": 0.25,
        "monthly_trend": 0.0,
        "inflow_ratio": (0.90, 1.05),
        "outflow_ratio": (0.68, 0.87),
        "tax_ratio": (0.05, 0.08),
        "debt_ratio": (0.08, 0.16),
        "amount_kopecks": 400_000_000,
        "term_months": 18,
    },
    "risky": {
        "inn": "7700000030",
        "revenue_volatility": 0.02,
        "monthly_trend": -0.12,
        "inflow_ratio": (0.80, 0.97),
        "outflow_ratio": (1.02, 1.20),
        "tax_ratio": (0.04, 0.07),
        "debt_ratio": (0.25, 0.40),
        "amount_kopecks": 600_000_000,
        "term_months": 12,
    },
}



def generate_month(
    month: str,
    revenue: int,
    rng: random.Random,
    settings: dict
) -> MonthlyFinancials:

    inflow = round(
        revenue * rng.uniform(*settings["inflow_ratio"])
    )

    outflow = round(
        inflow * rng.uniform(*settings["outflow_ratio"])
    )

    taxes = round(
        revenue * rng.uniform(*settings["tax_ratio"])
    )

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
    profile: str = "stable",
    overrides: Optional[dict] = None,
) -> ScoringInput:

    if profile not in PROFILES:
        raise ValueError("Неизвестный профиль: " + profile)

    settings = PROFILES[profile].copy()

    if overrides is not None:
        for key in overrides:
            if key not in settings:
                raise ValueError("Неизвестный параметр: " + key)

        settings.update(overrides)

    if reference_date is None:
        reference_date = date.today()

    rng = random.Random(seed)

    months = get_last_six_months(reference_date)

    base_revenue = rng.randint(
        80_000_000,
        150_000_000
    )

    financials = []

    for index, month in enumerate(months):

        trend = (1 + settings["monthly_trend"]) ** index

        volatility = rng.uniform(
            1 - settings["revenue_volatility"],
            1 + settings["revenue_volatility"]
        )

        revenue = round(
            base_revenue * trend * volatility
        )

        month_data = generate_month(
            month,
            revenue,
            rng,
            settings
        )

        financials.append(month_data)

    debt_payment = round(
        base_revenue * rng.uniform(
            *settings["debt_ratio"]
        )
    )

    return ScoringInput(
        inn=settings["inn"],
        amount_kopecks=settings["amount_kopecks"],
        term_months=settings["term_months"],
        annual_rate_bps=1990,
        application_date=reference_date,
        financials=financials,
        declared_monthly_debt_payment_kopecks=debt_payment,
    )



if __name__ == "__main__":
    for profile_name in PROFILES:

        company = generate_company_financials(
            profile=profile_name,
            seed=42
        )

        print("\nПрофиль:", profile_name)
        print(company.model_dump_json(indent=2))
