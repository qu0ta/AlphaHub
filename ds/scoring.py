from decimal import Decimal

from metrics import FinancialMetrics, calculate_financial_metrics
from schemas import ScoringInput, ScoringExplanation, ScoringResponse


MODEL_VERSION = "financial_rules_v1"


def calculate_score(metrics: FinancialMetrics) -> ScoringResponse:
    positive = []
    negative = []

    # 1. Покрытие платежей: максимум 60 баллов.
    coverage = metrics.debt_service_coverage_ratio

    if coverage is None:
        raise ValueError(
            "Нельзя рассчитать score: покрытие платежей не определено"
        )

    if coverage < Decimal("1"):
        coverage_points = 0
        negative.append(
            "Среднего денежного потока недостаточно "
            "для действующего и нового кредитных платежей"
        )
    elif coverage < Decimal("1.2"):
        coverage_points = 15
        negative.append(
            "Запас денежного потока над кредитными платежами менее 20%"
        )
    elif coverage < Decimal("1.5"):
        coverage_points = 30
        negative.append(
            "Ограниченный запас денежного потока "
            "над кредитными платежами: от 20% до 50%"
        )
    elif coverage < Decimal("2"):
        coverage_points = 45
        positive.append(
            "Денежный поток покрывает кредитные платежи "
            "с запасом не менее 50%"
        )
    else:
        coverage_points = 60
        positive.append(
            "Денежный поток покрывает кредитные платежи "
            "как минимум вдвое"
        )

    # 2. Стабильность выручки: максимум 20 баллов.
    cv = metrics.revenue_cv

    if cv is None:
        stability_points = 0
        negative.append(
            "Выручка за рассматриваемые шесть месяцев равна нулю"
        )
    elif cv <= Decimal("0.10"):
        stability_points = 20
        positive.append("Низкая изменчивость выручки")
    elif cv <= Decimal("0.25"):
        stability_points = 15
        positive.append("Умеренная изменчивость выручки")
    elif cv <= Decimal("0.50"):
        stability_points = 5
        negative.append("Значительные колебания выручки")
    else:
        stability_points = 0
        negative.append("Высокая изменчивость выручки")

    # 3. Динамика выручки: максимум 20 баллов.
    growth = metrics.revenue_growth_ratio

    if growth is None:
        growth_points = 0
        # При полностью нулевой выручке объяснение уже добавлено выше.
        if metrics.avg_monthly_revenue_kopecks > 0:
            negative.append(
                "Недостаточно истории выручки для оценки динамики: "
                "в первые три месяца выручка отсутствовала"
            )
    elif growth < Decimal("-0.20"):
        growth_points = 0
        negative.append(
            "Средняя выручка последних трёх месяцев снизилась "
            "более чем на 20% относительно первых трёх"
        )
    elif growth < Decimal("-0.05"):
        growth_points = 5
        negative.append(
            "Средняя выручка последних трёх месяцев снизилась "
            "более чем на 5%, но не более чем на 20%"
        )
    elif growth < Decimal("0.05"):
        growth_points = 10
        # Нейтральный результат: баллы есть, отдельный фактор не выводим.
    else:
        growth_points = 20
        positive.append(
            "Средняя выручка последних трёх месяцев выросла "
            "минимум на 5% относительно первых трёх"
        )

    score = coverage_points + stability_points + growth_points

    if coverage < Decimal("1"):
        score = min(score, 39)

    return ScoringResponse(
        score=score,
        explanation=ScoringExplanation(
            positive_factors=positive,
            negative_factors=negative,
        ),
        model_version=MODEL_VERSION,
    )


def score_application(data: ScoringInput) -> ScoringResponse:
    metrics = calculate_financial_metrics(data)
    return calculate_score(metrics)
