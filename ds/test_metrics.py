from datetime import date
from decimal import Decimal
import unittest

from pydantic import ValidationError

from schemas import ScoringInput, get_last_six_months
from metrics import (
    calculate_financial_metrics,
    calculate_monthly_payment,
)


def make_input(
    revenues_rubles: list[int] | None = None,
    outflow_rubles: int = 600_000,
) -> ScoringInput:
    application_date = date(2026, 10, 22)
    months = get_last_six_months(application_date)

    if revenues_rubles is None:
        revenues_rubles = [1_000_000] * 6

    if len(revenues_rubles) != 6:
        raise ValueError("Нужны 6 значений выручки")

    return ScoringInput(
        inn="6671123456",
        amount_kopecks=600_000 * 100,
        term_months=12,
        annual_rate_bps=0,
        application_date=application_date,
        declared_monthly_debt_payment_kopecks=50_000 * 100,
        financials=[
            {
                "period_month": month,
                "revenue_kopecks": revenue * 100,
                "operating_inflow_kopecks": 900_000 * 100,
                "operating_outflow_kopecks": outflow_rubles * 100,
                "tax_outflow_kopecks": 100_000 * 100,
            }
            for month, revenue in zip(months, revenues_rubles)
        ],
    )


class MetricsTests(unittest.TestCase):
    def test_basic_example(self):
        result = calculate_financial_metrics(
            make_input(),
        )

        self.assertEqual(
            result.avg_monthly_cashflow_kopecks,
            Decimal(200_000 * 100),
        )
        self.assertEqual(
            result.avg_monthly_revenue_kopecks,
            Decimal(1_000_000 * 100),
        )
        self.assertEqual(result.revenue_cv, Decimal(0))
        self.assertEqual(result.revenue_growth_ratio, Decimal(0))
        self.assertEqual(
            result.new_monthly_payment_kopecks,
            50_000 * 100,
        )
        self.assertEqual(
            result.total_monthly_debt_payment_kopecks,
            100_000 * 100,
        )
        self.assertEqual(
            result.debt_service_coverage_ratio,
            Decimal(2),
        )
        self.assertEqual(
            result.cash_after_debt_kopecks,
            Decimal(100_000 * 100),
        )
        self.assertEqual(result.flags, [])

    def test_revenue_growth_and_decline(self):
        cases = [
            ([100] * 3 + [120] * 3, Decimal("0.2")),
            ([100] * 3 + [80] * 3, Decimal("-0.2")),
        ]

        for revenues, expected in cases:
            with self.subTest(revenues=revenues):
                result = calculate_financial_metrics(
                    make_input(revenues),
                        )
                self.assertEqual(
                    result.revenue_growth_ratio,
                    expected,
                )

    def test_revenue_variation(self):
        # Среднее = 100, стандартное отклонение = 50.
        result = calculate_financial_metrics(
            make_input([50, 150, 50, 150, 50, 150]),
        )
        self.assertEqual(result.revenue_cv, Decimal("0.5"))

    def test_zero_revenue(self):
        result = calculate_financial_metrics(
            make_input([0] * 6),
        )

        self.assertIsNone(result.revenue_cv)
        self.assertIsNone(result.revenue_growth_ratio)
        self.assertIn("ZERO_REVENUE", result.flags)
        self.assertIn("ZERO_REVENUE_BASE", result.flags)

    def test_growth_from_zero(self):
        result = calculate_financial_metrics(
            make_input([0, 0, 0, 100, 100, 100]),
        )

        self.assertIsNone(result.revenue_growth_ratio)
        self.assertEqual(
            result.revenue_change_kopecks,
            Decimal(100 * 100),
        )

    def test_negative_cashflow(self):
        result = calculate_financial_metrics(
            make_input(outflow_rubles=1_000_000),
        )

        self.assertEqual(
            result.avg_monthly_cashflow_kopecks,
            Decimal(-200_000 * 100),
        )
        self.assertEqual(
            result.debt_service_coverage_ratio,
            Decimal(-2),
        )

    def test_payment_with_interest(self):
        # 12% годовых = 1% за месяц.
        # Кредит 1000 ₽ на месяц: платёж 1010 ₽.
        payment = calculate_monthly_payment(
            amount_kopecks=1000 * 100,
            term_months=1,
            annual_rate_bps=1200,
        )
        self.assertEqual(payment, 1010 * 100)

    def test_duplicate_month_rejected(self):
        payload = make_input().model_dump()
        payload["financials"][1]["period_month"] = (
            payload["financials"][0]["period_month"]
        )

        with self.assertRaises(ValidationError):
            ScoringInput.model_validate(payload)

    def test_year_boundary(self):
        self.assertEqual(
            get_last_six_months(date(2026, 1, 15)),
            [
                "2025-07",
                "2025-08",
                "2025-09",
                "2025-10",
                "2025-11",
                "2025-12",
            ],
        )


if __name__ == "__main__":
    unittest.main()
