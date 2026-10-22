import unittest
from dataclasses import replace
from decimal import Decimal

from metrics import calculate_financial_metrics
from scoring import calculate_score, score_application
from test_metrics import make_input


class ScoringTests(unittest.TestCase):
    def setUp(self):
        self.base = calculate_financial_metrics(
            make_input(),
        )

    def test_basic_score(self):
        response = score_application(
            make_input(),
        )

        self.assertEqual(response.score, 90)
        self.assertEqual(len(response.explanation.positive_factors), 2)
        self.assertEqual(response.explanation.negative_factors, [])
        self.assertEqual(
            response.model_version,
            "financial_rules_v1",
        )

    def test_coverage_boundaries(self):
        # Стабильность и динамика вместе дают 30 баллов.
        cases = [
            ("0.9999", 30),
            ("1", 45),
            ("1.1999", 45),
            ("1.2", 60),
            ("1.4999", 60),
            ("1.5", 75),
            ("1.9999", 75),
            ("2", 90),
        ]

        for coverage, expected_score in cases:
            with self.subTest(coverage=coverage):
                metrics = replace(
                    self.base,
                    debt_service_coverage_ratio=Decimal(coverage),
                )
                self.assertEqual(
                    calculate_score(metrics).score,
                    expected_score,
                )

    def test_stability_boundaries(self):
        # Покрытие и динамика вместе дают 70 баллов.
        cases = [
            ("0", 90),
            ("0.10", 90),
            ("0.1001", 85),
            ("0.25", 85),
            ("0.2501", 75),
            ("0.50", 75),
            ("0.5001", 70),
        ]

        for cv, expected_score in cases:
            with self.subTest(cv=cv):
                metrics = replace(
                    self.base,
                    revenue_cv=Decimal(cv),
                )
                self.assertEqual(
                    calculate_score(metrics).score,
                    expected_score,
                )

    def test_growth_boundaries(self):
        # Покрытие и стабильность вместе дают 80 баллов.
        cases = [
            ("-0.2001", 80),
            ("-0.20", 85),
            ("-0.0501", 85),
            ("-0.05", 90),
            ("0", 90),
            ("0.0499", 90),
            ("0.05", 100),
        ]

        for growth, expected_score in cases:
            with self.subTest(growth=growth):
                metrics = replace(
                    self.base,
                    revenue_growth_ratio=Decimal(growth),
                )
                self.assertEqual(
                    calculate_score(metrics).score,
                    expected_score,
                )

    def test_insufficient_coverage_caps_score(self):
        # Без ограничения получилось бы 0 + 20 + 20 = 40.
        metrics = replace(
            self.base,
            debt_service_coverage_ratio=Decimal("0.9"),
            revenue_cv=Decimal("0"),
            revenue_growth_ratio=Decimal("0.10"),
        )

        response = calculate_score(metrics)

        self.assertEqual(response.score, 39)
        self.assertTrue(response.explanation.negative_factors)

    def test_undefined_coverage_raises_error(self):
        metrics = replace(
            self.base,
            debt_service_coverage_ratio=None,
        )

        with self.assertRaises(ValueError):
            calculate_score(metrics)

    def test_zero_revenue_explanation(self):
        response = score_application(
            make_input([0] * 6),
        )

        self.assertEqual(response.score, 60)
        self.assertIn(
            "Выручка за рассматриваемые шесть месяцев равна нулю",
            response.explanation.negative_factors,
        )
        self.assertNotIn(
            "Низкая изменчивость выручки",
            response.explanation.positive_factors,
        )

    def test_negative_cashflow(self):
        response = score_application(
            make_input(outflow_rubles=1_000_000),
        )

        self.assertEqual(response.score, 30)
        self.assertIn(
            "Среднего денежного потока недостаточно "
            "для действующего и нового кредитных платежей",
            response.explanation.negative_factors,
        )


if __name__ == "__main__":
    unittest.main()
