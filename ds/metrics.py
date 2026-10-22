from dataclasses import dataclass
from decimal import Decimal, ROUND_HALF_UP, localcontext

from schemas import ScoringInput


@dataclass(frozen=True)
class FinancialMetrics:
    monthly_cashflows_kopecks: list[int]
    avg_monthly_cashflow_kopecks: Decimal
    avg_monthly_revenue_kopecks: Decimal

    revenue_cv: Decimal | None
    revenue_growth_ratio: Decimal | None
    revenue_change_kopecks: Decimal

    new_monthly_payment_kopecks: int
    total_monthly_debt_payment_kopecks: int
    debt_service_coverage_ratio: Decimal | None
    cash_after_debt_kopecks: Decimal

    flags: list[str]


def average(values: list[int]) -> Decimal:
    if not values:
        raise ValueError("Для среднего нужен непустой список")

    return Decimal(sum(values)) / Decimal(len(values))


def calculate_monthly_payment(
    amount_kopecks: int,
    term_months: int,
    annual_rate_bps: int,
) -> int:
    """Расчётный аннуитетный платёж в копейках, без комиссий."""
    if amount_kopecks <= 0:
        raise ValueError("Сумма кредита должна быть положительной")
    if term_months <= 0:
        raise ValueError("Срок должен быть положительным")
    if annual_rate_bps < 0:
        raise ValueError("Ставка не может быть отрицательной")

    with localcontext() as ctx:
        ctx.prec = 40

        amount = Decimal(amount_kopecks)
        rate = Decimal(annual_rate_bps) / Decimal(120_000)

        if rate == 0:
            payment = amount / Decimal(term_months)
        else:
            payment = (
                amount * rate
                / (Decimal(1) - (Decimal(1) + rate) ** (-term_months))
            )

        return int(
            payment.quantize(Decimal("1"), rounding=ROUND_HALF_UP)
        )


def calculate_financial_metrics(
    data: ScoringInput,
) -> FinancialMetrics:
    with localcontext() as ctx:
        ctx.prec = 40

        records = sorted(
            data.financials,
            key=lambda record: record.period_month,
        )

        revenues = [record.revenue_kopecks for record in records]

        # Поток до платежей по действующим и новому кредитам.
        cashflows = [
            record.operating_inflow_kopecks
            - record.operating_outflow_kopecks
            - record.tax_outflow_kopecks
            for record in records
        ]

        avg_revenue = average(revenues)
        avg_cashflow = average(cashflows)
        flags = []

        # Коэффициент вариации: колебания относительно среднего.
        if avg_revenue == 0:
            revenue_cv = None
            flags.append("ZERO_REVENUE")
        else:
            variance = sum(
                (Decimal(value) - avg_revenue) ** 2
                for value in revenues
            ) / Decimal(len(revenues))

            revenue_cv = variance.sqrt() / avg_revenue

        # Динамика: последние 3 месяца относительно первых 3.
        early_revenue = average(revenues[:3])
        recent_revenue = average(revenues[3:])
        revenue_change = recent_revenue - early_revenue

        if early_revenue == 0:
            growth = None
            flags.append("ZERO_REVENUE_BASE")
        else:
            growth = revenue_change / early_revenue

        new_payment = calculate_monthly_payment(
            amount_kopecks=data.amount_kopecks,
            term_months=data.term_months,
            annual_rate_bps=data.annual_rate_bps,
        )

        total_payment = (
            data.declared_monthly_debt_payment_kopecks
            + new_payment
        )

        if total_payment == 0:
            coverage = None
            flags.append("ZERO_TOTAL_DEBT_PAYMENT")
        else:
            coverage = avg_cashflow / Decimal(total_payment)

        return FinancialMetrics(
            monthly_cashflows_kopecks=cashflows,
            avg_monthly_cashflow_kopecks=avg_cashflow,
            avg_monthly_revenue_kopecks=avg_revenue,
            revenue_cv=revenue_cv,
            revenue_growth_ratio=growth,
            revenue_change_kopecks=revenue_change,
            new_monthly_payment_kopecks=new_payment,
            total_monthly_debt_payment_kopecks=total_payment,
            debt_service_coverage_ratio=coverage,
            cash_after_debt_kopecks=(
                avg_cashflow - Decimal(total_payment)
            ),
            flags=flags,
        )
