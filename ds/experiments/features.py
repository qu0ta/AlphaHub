import math

import numpy as np

from metrics import calculate_financial_metrics
from schemas import ScoringInput


FEATURE_NAMES = [
    "avg_monthly_revenue_kopecks", "avg_operating_cashflow_kopecks", "revenue_cv",
    "revenue_growth_ratio", "existing_monthly_debt_payment_kopecks", "existing_debt_to_revenue",
    "cashflow_to_existing_debt", "new_monthly_payment_kopecks", "debt_service_coverage_ratio",
    "loan_to_monthly_revenue", "term_months", "annual_rate",
]


def extract_features(data: ScoringInput) -> list[float]:
    metrics = calculate_financial_metrics(data)
    if metrics.debt_service_coverage_ratio is None:
        raise ValueError("Покрытие платежей не определено")
    revenue = float(metrics.avg_monthly_revenue_kopecks)
    cashflow = float(metrics.avg_monthly_cashflow_kopecks)
    existing = data.declared_monthly_debt_payment_kopecks
    return [
        revenue, cashflow,
        float(metrics.revenue_cv) if metrics.revenue_cv is not None else math.nan,
        float(metrics.revenue_growth_ratio) if metrics.revenue_growth_ratio is not None else math.nan,
        existing, existing / revenue if revenue else math.nan,
        cashflow / existing if existing else math.nan, metrics.new_monthly_payment_kopecks,
        float(metrics.debt_service_coverage_ratio), data.amount_kopecks / revenue if revenue else math.nan,
        data.term_months, data.annual_rate_bps / 10_000,
    ]


def feature_matrix(rows):
    # Whitelist computed exclusively from ScoringInput: no profile, target or future outcomes.
    return np.asarray([extract_features(row.scoring_input) for row in rows], dtype=float)
