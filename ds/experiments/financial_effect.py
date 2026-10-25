from decimal import Decimal, ROUND_HALF_UP

import numpy as np

from .config import Economics


def money(amount: int, rate: float) -> int:
    return int((Decimal(int(amount)) * Decimal(str(rate))).quantize(Decimal("1"), rounding=ROUND_HALF_UP))


def calculate_financials(y, predicted_default, amounts, economics: Economics):
    fp_amount = int(np.sum(amounts[(y == 0) & (predicted_default == 1)]))
    fn_amount = int(np.sum(amounts[(y == 1) & (predicted_default == 0)]))
    tn_amount = int(np.sum(amounts[(y == 0) & (predicted_default == 0)]))
    default_loss = money(fn_amount, economics.lgd)
    opportunity_loss = money(fp_amount, economics.net_yield)
    cost = default_loss + opportunity_loss
    return {
        "fp_amount_kopecks": fp_amount, "fn_amount_kopecks": fn_amount,
        "fn_default_loss_kopecks": default_loss, "fp_opportunity_loss_kopecks": opportunity_loss,
        "total_error_cost_kopecks": cost, "cost_per_application_kopecks": cost / len(y),
        "performing_net_income_kopecks": money(tn_amount, economics.net_yield),
        "approved_portfolio_profit_kopecks": money(tn_amount, economics.net_yield) - default_loss,
        "manual_review_cost_applied_kopecks": 0,
    }


def compare_financials(rules, model):
    baseline = rules["total_error_cost_kopecks"]
    effect = baseline - model["total_error_cost_kopecks"]
    return {"ml_effect_kopecks": effect,
            "relative_loss_reduction_percent": 100 * effect / baseline if baseline else None}


def tune_threshold(y, probabilities, amounts, economics: Economics):
    """Minimize the specified FP/FN money cost on validation, including tied PDs."""
    order = np.argsort(probabilities, kind="stable")
    ordered = probabilities[order]
    good = np.r_[np.int64(0), np.cumsum(np.where(y[order] == 0, amounts[order], 0), dtype=np.int64)]
    bad = np.r_[np.int64(0), np.cumsum(np.where(y[order] == 1, amounts[order], 0), dtype=np.int64)]
    candidates = np.unique(np.r_[0.0, probabilities, np.nextafter(float(np.max(probabilities)), np.inf)])
    best, curve = None, []
    for threshold in candidates:
        boundary = int(np.searchsorted(ordered, threshold, side="left"))
        cost = money(int(bad[boundary]), economics.lgd) + money(int(good[-1] - good[boundary]), economics.net_yield)
        # Equality with the threshold predicts default. On ties approve fewer.
        key = (cost, boundary)
        curve.append({"threshold": float(threshold), "cost_kopecks": cost,
                      "approval_rate": boundary / len(y)})
        if best is None or key < best[0]:
            best = (key, float(threshold))
    return best[1], curve


def sensitivity_analysis(y, rules_prediction, model_prediction, amounts):
    result = []
    for lgd in (0.25, 0.45, 0.65):
        for net_yield in (0.04, 0.08, 0.12):
            economics = Economics(net_yield=net_yield, lgd=lgd)
            rules = calculate_financials(y, rules_prediction, amounts, economics)
            model = calculate_financials(y, model_prediction, amounts, economics)
            result.append({"lgd": lgd, "net_yield": net_yield,
                           "rules_cost_kopecks": rules["total_error_cost_kopecks"],
                           "ml_cost_kopecks": model["total_error_cost_kopecks"],
                           **compare_financials(rules, model)})
    return result
