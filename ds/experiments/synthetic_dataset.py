import csv
import math
import random
from collections import Counter
from datetime import date
from pathlib import Path
from typing import Literal

import numpy as np
from pydantic import BaseModel, ConfigDict, Field

from metrics import calculate_financial_metrics
from schemas import MonthlyFinancials, ScoringInput, get_last_six_months

from .config import Scenario


PROFILES = {
    "stable": {"volatility": 0.04, "trend": (-0.01, 0.025), "expenses": (0.50, 0.66), "debt": (0.015, 0.055)},
    "medium": {"volatility": 0.16, "trend": (-0.035, 0.035), "expenses": (0.65, 0.84), "debt": (0.04, 0.11)},
    "risky": {"volatility": 0.30, "trend": (-0.08, 0.02), "expenses": (0.78, 1.08), "debt": (0.08, 0.18)},
}


class SyntheticApplication(BaseModel):
    model_config = ConfigDict(extra="forbid")
    application_id: str
    scoring_input: ScoringInput
    default: int = Field(ge=0, le=1, strict=True)
    profile: Literal["stable", "medium", "risky"]
    scenario: Literal["base", "high_noise", "stress"]


def sample_default(data: ScoringInput, rng: random.Random, scenario: Scenario) -> int:
    """Latent stochastic mechanism, independent of scoring.py and profile labels."""
    metrics = calculate_financial_metrics(data)
    coverage = float(metrics.debt_service_coverage_ratio)
    revenue = float(metrics.avg_monthly_revenue_kopecks)
    margin = float(metrics.avg_monthly_cashflow_kopecks) / revenue
    debt_ratio = data.declared_monthly_debt_payment_kopecks / revenue
    signed_coverage = math.copysign(math.log1p(abs(coverage)), coverage)
    hidden_shock = 2.2 if rng.random() < 0.08 else 0.0
    nonlinear_risk = 0.65 if margin < 0.05 and debt_ratio > 0.08 else 0.0
    logit = (
        -1.8 - 0.9 * signed_coverage
        + 0.9 * float(metrics.revenue_cv) - 0.7 * float(metrics.revenue_growth_ratio)
        + 0.45 * debt_ratio + 0.20 * (data.term_months / 12 - 1)
        + nonlinear_risk + hidden_shock + rng.gauss(0, scenario.hidden_risk_std)
        + scenario.risk_shift
    )
    probability = 1 / (1 + math.exp(-max(-30, min(30, logit))))
    # The true probability and hidden risk are intentionally not returned.
    return int(rng.random() < probability)


def generate_dataset(count: int, seed: int, scenario_name: str, scenario: Scenario,
                     namespace: str) -> list[SyntheticApplication]:
    if count < 1 or scenario_name not in {"base", "high_noise", "stress"}:
        raise ValueError("Нужны положительный count и известный сценарий")
    rng = random.Random(seed)
    rows = []
    for index in range(count):
        profile_name = rng.choices(list(PROFILES), weights=[0.4, 0.4, 0.2])[0]
        profile = PROFILES[profile_name]
        base_revenue = rng.randint(40_000_000, 350_000_000)
        trend = rng.uniform(*profile["trend"])
        expenses = rng.uniform(*profile["expenses"])
        volatility = profile["volatility"] * scenario.volatility_multiplier
        application_date = date(2025, rng.randint(1, 12), 15)
        phase = rng.uniform(0, 2 * math.pi)
        latent_months, observed_months = [], []
        for month_index, month in enumerate(get_last_six_months(application_date)):
            calendar_month = int(month[-2:])
            seasonal = 1 + scenario.seasonal_amplitude * math.sin(2 * math.pi * calendar_month / 12 + phase)
            shock = 1.0
            if rng.random() < scenario.shock_probability:
                # Both one-off positive and negative financial shocks occur.
                shock = math.exp(rng.choice([-1, 1]) * scenario.shock_scale)
            revenue = max(1, round(base_revenue * (1 + trend) ** month_index * seasonal * shock
                                   * rng.lognormvariate(-volatility**2 / 2, volatility)))
            inflow = max(1, round(revenue * rng.uniform(0.90, 1.08) * scenario.inflow_multiplier))
            outflow = max(0, round(inflow * max(0.05, expenses + rng.gauss(0, volatility / 4))
                                   * scenario.expense_multiplier))
            taxes = round(revenue * rng.uniform(0.04, 0.07))
            latent = MonthlyFinancials(
                period_month=month, revenue_kopecks=revenue, operating_inflow_kopecks=inflow,
                operating_outflow_kopecks=outflow, tax_outflow_kopecks=taxes,
            )
            latent_months.append(latent)
            observed = latent.model_dump()
            for key in observed:
                if key != "period_month":
                    noise = scenario.observation_noise
                    observed[key] = round(observed[key] * rng.lognormvariate(-noise**2 / 2, noise))
            observed_months.append(MonthlyFinancials(**observed))
        data = ScoringInput(
            inn=f"SYNTH-{namespace}-{index:08d}",
            amount_kopecks=round(base_revenue * rng.uniform(1.2, 4.5)),
            term_months=rng.choice([12, 18, 24]), annual_rate_bps=rng.choice([1790, 1990, 2290, 2590]),
            application_date=application_date, financials=observed_months,
            declared_monthly_debt_payment_kopecks=round(base_revenue * rng.uniform(*profile["debt"])),
        )
        outcome = sample_default(data.model_copy(update={"financials": latent_months}), rng, scenario)
        rows.append(SyntheticApplication(application_id=f"{namespace}-{index:08d}", scoring_input=data,
                                         default=outcome, profile=profile_name, scenario=scenario_name))
    return rows


def describe_dataset(rows):
    from .features import FEATURE_NAMES, feature_matrix
    features = feature_matrix(rows)
    rate = sum(row.default for row in rows) / len(rows)
    return {
        "count": len(rows), "unique_applications": len({r.application_id for r in rows}),
        "unique_clients": len({r.scoring_input.inn for r in rows}), "default_rate": rate,
        "base_default_rate_in_target_band": 0.10 <= rate <= 0.25 if rows[0].scenario == "base" else None,
        "profiles": dict(Counter(r.profile for r in rows)),
        "feature_distributions": {
            name: {"missing": int(np.sum(np.isnan(features[:, index]))),
                   "p05": float(np.nanquantile(features[:, index], 0.05)),
                   "median": float(np.nanmedian(features[:, index])),
                   "p95": float(np.nanquantile(features[:, index], 0.95))}
            for index, name in enumerate(FEATURE_NAMES)
        },
    }


def save_dataset(rows, path: Path):
    from .features import FEATURE_NAMES, extract_features
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", newline="", encoding="utf-8") as stream:
        writer = csv.DictWriter(stream, fieldnames=["application_id", "profile", "scenario", "default",
                                                    "scoring_input_json", *FEATURE_NAMES])
        writer.writeheader()
        for row in rows:
            writer.writerow({"application_id": row.application_id, "profile": row.profile,
                             "scenario": row.scenario, "default": row.default,
                             "scoring_input_json": row.scoring_input.model_dump_json(),
                             **dict(zip(FEATURE_NAMES, extract_features(row.scoring_input)))})
