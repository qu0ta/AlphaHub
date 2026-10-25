import numpy as np
from sklearn.metrics import (
    accuracy_score, average_precision_score, brier_score_loss, confusion_matrix,
    f1_score, precision_score, recall_score, roc_auc_score,
)

from .features import feature_matrix
from .financial_effect import calculate_financials, compare_financials, sensitivity_analysis
from .rule_baseline import evaluate_rules


def classification_metrics(y, predicted, risk, *, probability=False):
    tn, fp, fn, tp = (int(value) for value in confusion_matrix(y, predicted, labels=[0, 1]).ravel())
    both_classes = len(np.unique(y)) == 2
    return {
        "accuracy": float(accuracy_score(y, predicted)),
        "precision": float(precision_score(y, predicted, zero_division=0)),
        "recall": float(recall_score(y, predicted, zero_division=0)),
        "f1": float(f1_score(y, predicted, zero_division=0)),
        "roc_auc": float(roc_auc_score(y, risk)) if both_classes else None,
        "pr_auc": float(average_precision_score(y, risk)) if both_classes else None,
        "confusion_matrix": {"tn": tn, "fp": fp, "fn": fn, "tp": tp},
        "false_positive_rate": fp / (fp + tn) if fp + tn else None,
        "false_negative_rate": fn / (fn + tp) if fn + tp else None,
        "approval_rate": (tn + fn) / len(y),
        "approved_default_rate": fn / (tn + fn) if tn + fn else None,
        "brier_score": float(brier_score_loss(y, risk)) if probability else None,
    }


def evaluate_dataset(rows, pipeline, threshold: float, economics):
    y = np.asarray([row.default for row in rows], dtype=int)
    amounts = np.asarray([row.scoring_input.amount_kopecks for row in rows], dtype=np.int64)
    rules = evaluate_rules(rows)
    probabilities = pipeline.predict_proba(feature_matrix(rows))[:, 1]
    predicted = (probabilities >= threshold).astype(int)
    metrics = {
        "rules": classification_metrics(y, rules["predicted_default"], rules["risk_ranking"]),
        "logistic_regression": classification_metrics(y, predicted, probabilities, probability=True),
    }
    financials = {"rules": calculate_financials(y, rules["predicted_default"], amounts, economics),
                  "logistic_regression": calculate_financials(y, predicted, amounts, economics)}
    result = {
        "count": len(rows), "default_rate": float(np.mean(y)), "ml_threshold": threshold,
        "metrics": metrics, "financials": financials,
        "financial_comparison": compare_financials(financials["rules"], financials["logistic_regression"]),
        "three_level_rule_decisions": rules["three_level_distribution"],
        "sensitivity_fixed_threshold": sensitivity_analysis(y, rules["predicted_default"], predicted, amounts),
    }
    predictions = [
        {"application_id": row.application_id, "inn": row.scoring_input.inn,
         "scenario": row.scenario, "default": row.default, "amount_kopecks": row.scoring_input.amount_kopecks,
         "rule_score": int(rules["scores"][index]), "rule_original_decision": str(rules["bands"][index]),
         "rule_predicted_default": int(rules["predicted_default"][index]),
         "ml_probability": float(probabilities[index]), "ml_predicted_default": int(predicted[index])}
        for index, row in enumerate(rows)
    ]
    return result, predictions
