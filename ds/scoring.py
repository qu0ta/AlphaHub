from decimal import Decimal

from metrics import FinancialMetrics, calculate_financial_metrics
from schemas import (
    ScoringInput,
    ScoringExplanation,
    ScoringResponse,
    ScoreComponents,
    DetailedScoringResponse,
)


MODEL_VERSION = "financial_rules_v2"

# Плавная шкала: баллы линейно интерполируются между опорными точками,
# а не прыгают ступенькой на границе диапазона. Опорные точки совпадают
# с прежними порогами, поэтому смысл диапазонов (и тексты объяснений) сохранён.
COVERAGE_ANCHORS = (
    (Decimal("1.0"), 15),
    (Decimal("1.2"), 30),
    (Decimal("1.5"), 45),
    (Decimal("2.0"), 60),
)
STABILITY_ANCHORS = (
    (Decimal("0.10"), 20),
    (Decimal("0.25"), 15),
    (Decimal("0.50"), 5),
    (Decimal("0.75"), 0),
)
GROWTH_ANCHORS = (
    (Decimal("-0.20"), 0),
    (Decimal("-0.05"), 5),
    (Decimal("0"), 10),  # выручка без изменений — нейтральные 10 баллов, как раньше
    (Decimal("0.20"), 20),
)


def interpolate_points(value: Decimal, anchors) -> int:
    """Кусочно-линейная интерполяция; за краями — значение крайней точки."""
    if value <= anchors[0][0]:
        return anchors[0][1]
    for (x0, y0), (x1, y1) in zip(anchors, anchors[1:]):
        if value <= x1:
            share = (value - x0) / (x1 - x0)
            points = Decimal(y0) + (Decimal(y1) - Decimal(y0)) * share
            return int(points.quantize(Decimal("1"), rounding="ROUND_HALF_UP"))
    return anchors[-1][1]


def calculate_score_from_metrics(
    metrics: FinancialMetrics,
) -> DetailedScoringResponse:
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

    # Тексты выше выбираются по диапазонам, а баллы — по плавной шкале.
    # Покрытие < 1 остаётся жёстким: 0 баллов и потолок score 39.
    if coverage >= Decimal("1"):
        coverage_points = interpolate_points(coverage, COVERAGE_ANCHORS)
    if cv is not None:
        stability_points = interpolate_points(cv, STABILITY_ANCHORS)
    if growth is not None:
        growth_points = interpolate_points(growth, GROWTH_ANCHORS)

    components = ScoreComponents(
        coverage_points=coverage_points,
        stability_points=stability_points,
        growth_points=growth_points,
    )

    raw_score = sum(components.model_dump().values())

    score_cap = 39 if coverage < Decimal("1") else None

    score = (
        min(raw_score, score_cap)
        if score_cap is not None
        else raw_score
    )

    return DetailedScoringResponse(
        score=score,
        components=components,
        raw_score=raw_score,
        score_cap=score_cap,
        explanation=ScoringExplanation(
            positive_factors=positive,
            negative_factors=negative,
        ),
        model_version=MODEL_VERSION,
    )


def calculate_score(
    data: ScoringInput,
) -> DetailedScoringResponse:
    metrics = calculate_financial_metrics(data)
    return calculate_score_from_metrics(metrics)


def score_application(
    data: ScoringInput,
) -> ScoringResponse:
    result = calculate_score(data)

    return ScoringResponse(
        score=result.score,
        explanation=result.explanation,
        model_version=result.model_version,
    )
