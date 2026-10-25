import os

os.environ.setdefault("MPLCONFIGDIR", "/tmp/alphahub-experiments-matplotlib")
import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np
from sklearn.calibration import calibration_curve
from sklearn.metrics import ConfusionMatrixDisplay, RocCurveDisplay

from .features import feature_matrix


ALGORITHMS = ("rules", "logistic_regression")
LABELS = {"rules": "Правила", "logistic_regression": "Logistic Regression"}
SCENARIOS = {"base": "Базовый test", "high_noise": "Высокий шум", "stress": "Стресс"}
COLORS = ["#61758a", "#dc242a"]


def create_figures(manifest, rows, predictions, curve, output):
    plt.rcParams.update({"font.family": "DejaVu Sans", "font.size": 10})
    figures = []

    def save(figure, name):
        figure.savefig(output / f"{name}.png", dpi=150)
        figure.savefig(output / f"{name}.svg")
        plt.close(figure)
        figures.append(f"{name}.png")

    y = np.asarray([row["default"] for row in predictions])
    ml_risk = np.asarray([row["ml_probability"] for row in predictions])
    rule_risk = 1 - np.asarray([row["rule_score"] for row in predictions]) / 100
    primary = manifest["runs"][0]["scenarios"]["base"]
    features = feature_matrix(rows)
    figure, axes = plt.subplots(2, 2, figsize=(11, 7), constrained_layout=True)
    for axis, index, label in zip(axes.ravel(), (0, 2, 8), ("Выручка, млн ₽/мес.", "CV выручки", "Покрытие всех платежей")):
        values = features[:, index] / 100_000_000 if index == 0 else features[:, index]
        lower, upper = np.nanquantile(values, [0.01, 0.99])
        bins = np.linspace(lower, upper if upper > lower else lower + 1, 25)
        for target in (0, 1):
            axis.hist(values[y == target], bins=bins, density=True, alpha=0.55,
                      color=COLORS[target], label=f"default={target}")
        axis.set(xlabel=label, ylabel="Плотность", title="Независимый базовый test; диапазон p01–p99")
        axis.legend()
    profile_names = ("stable", "medium", "risky")
    profile_rates = [np.mean([r.default for r in rows if r.profile == name]) for name in profile_names]
    axes[1, 1].bar(profile_names, profile_rates, color="#61758a")
    axes[1, 1].set(title="Доля дефолтов по профилям; профиль не является признаком", ylabel="Доля дефолтов", ylim=(0, 1))
    save(figure, "client_distributions")

    figure, axis = plt.subplots(figsize=(7, 6), constrained_layout=True)
    RocCurveDisplay.from_predictions(y, rule_risk, name="Правила", ax=axis, curve_kwargs={"color": COLORS[0]})
    RocCurveDisplay.from_predictions(y, ml_risk, name="Logistic Regression", ax=axis, curve_kwargs={"color": COLORS[1]})
    axis.plot([0, 1], [0, 1], "--", color="gray")
    axis.set(title="ROC на базовом test", xlabel="FPR: доля ложных отказов", ylabel="Recall: найденные дефолты")
    save(figure, "roc_curve")

    figure, axes = plt.subplots(1, 2, figsize=(11, 5), constrained_layout=True)
    for axis, algorithm in zip(axes, ALGORITHMS):
        cm = primary["metrics"][algorithm]["confusion_matrix"]
        ConfusionMatrixDisplay(np.array([[cm["tn"], cm["fp"]], [cm["fn"], cm["tp"]]]),
                               display_labels=["Недефолт 0", "Дефолт 1"]).plot(ax=axis, colorbar=False, cmap="Blues")
        axis.set(title=LABELS[algorithm], xlabel="Прогноз: 1 = отказ", ylabel="Фактический исход")
    save(figure, "confusion_matrix")

    figure, axis = plt.subplots(figsize=(8, 5), constrained_layout=True)
    positions = np.arange(2)
    for index, algorithm in enumerate(ALGORITHMS):
        cm = primary["metrics"][algorithm]["confusion_matrix"]
        axis.bar(positions + (index - 0.5) * 0.35, [cm["fp"], cm["fn"]], width=0.35,
                 label=LABELS[algorithm], color=COLORS[index])
    axis.set(xticks=positions, xticklabels=["FP: ложные отказы", "FN: дефолты одобрены"], ylabel="Заявки", title="Ошибки на одинаковом базовом test")
    axis.legend()
    save(figure, "fp_fn_comparison")

    figure, axis = plt.subplots(figsize=(8, 5), constrained_layout=True)
    financials = primary["financials"]
    fn_losses = [financials[name]["fn_default_loss_kopecks"] / 100_000_000 for name in ALGORITHMS]
    fp_losses = [financials[name]["fp_opportunity_loss_kopecks"] / 100_000_000 for name in ALGORITHMS]
    axis.bar([LABELS[name] for name in ALGORITHMS], fn_losses, label="FN: убыток при дефолте", color=COLORS[1])
    axis.bar([LABELS[name] for name in ALGORITHMS], fp_losses, bottom=fn_losses,
             label="FP: упущенная прибыль", color=COLORS[0])
    axis.set(title="Стоимость ошибок: базовый test", ylabel="Условные потери, млн ₽")
    axis.legend()
    save(figure, "financial_effect")

    figure, axis = plt.subplots(figsize=(8, 5), constrained_layout=True)
    axis.plot([point["threshold"] for point in curve], [point["cost_kopecks"] / 100_000_000 for point in curve], color=COLORS[1])
    threshold = manifest["runs"][0]["ml_threshold"]
    axis.axvline(threshold, linestyle="--", color="gray", label=f"Выбранный порог {threshold:.4f}")
    axis.set(title="Настройка порога на validation", xlabel="Порог PD; равенство = отказ", ylabel="Стоимость FP + FN, млн ₽")
    axis.legend()
    save(figure, "threshold_cost")

    figure, axes = plt.subplots(1, 3, figsize=(14, 5), constrained_layout=True)
    positions = np.arange(3)
    for axis, metric, label in zip(axes, ("roc_auc", "false_negative_rate", "cost_per_application_kopecks"),
                                    ("ROC-AUC", "FNR: пропущенные дефолты", "Потери на заявку, ₽")):
        for index, algorithm in enumerate(ALGORITHMS):
            stats = [manifest["aggregate"][scenario][algorithm][metric] for scenario in SCENARIOS]
            divisor = 100 if metric.endswith("kopecks") else 1
            axis.bar(positions + (index - 0.5) * 0.35, [s["mean"] / divisor for s in stats], width=0.35,
                     yerr=[s["std"] / divisor for s in stats], capsize=4, label=LABELS[algorithm], color=COLORS[index])
        axis.set(xticks=positions, xticklabels=list(SCENARIOS.values()), ylabel=label, title="Среднее ± SD по seed")
        axis.tick_params(axis="x", rotation=20)
    axes[0].legend()
    save(figure, "robustness_scenarios")

    figure, axis = plt.subplots(figsize=(7, 6), constrained_layout=True)
    observed, predicted = calibration_curve(y, ml_risk, n_bins=8, strategy="quantile")
    axis.plot(predicted, observed, "o-", color=COLORS[1], label="Logistic Regression")
    axis.plot([0, 1], [0, 1], "--", color="gray")
    axis.set(xlabel="Средняя предсказанная PD", ylabel="Фактическая доля дефолтов", title="Калибровка на базовом test", xlim=(0, 1), ylim=(0, 1))
    axis.legend()
    save(figure, "calibration")
    return figures
