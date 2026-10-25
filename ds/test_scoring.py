import unittest
from dataclasses import replace
from decimal import Decimal

from metrics import calculate_financial_metrics
from scoring import (
    calculate_score_from_metrics as calculate_score,
    score_application,
)
from test_metrics import make_input


class ScoringTests(unittest.TestCase):
    def setUp(self):
        self.base = calculate_financial_metrics(
            make_input(),
        )

    def components(self, **changes):
        metrics = replace(self.base, **changes)
        return calculate_score(metrics).components

    def test_basic_score(self):
        response = score_application(
            make_input(),
        )

        self.assertEqual(response.score, 90)
        self.assertEqual(len(response.explanation.positive_factors), 2)
        self.assertEqual(response.explanation.negative_factors, [])
        self.assertEqual(
            response.model_version,
            "financial_rules_v2",
        )

    def test_coverage_scale(self):
        # Плавная шкала: 15 баллов при C=1, 60 при C>=2, между опорными точками — линейно.
        cases = [
            ("0.9999", 0),
            ("1", 15),
            ("1.1", 23),
            ("1.2", 30),
            ("1.35", 38),
            ("1.5", 45),
            ("1.75", 53),
            ("2", 60),
            ("3", 60),
        ]

        for coverage, expected in cases:
            with self.subTest(coverage=coverage):
                points = self.components(
                    debt_service_coverage_ratio=Decimal(coverage),
                ).coverage_points
                self.assertEqual(points, expected)

    def test_stability_scale(self):
        cases = [
            ("0", 20),
            ("0.10", 20),
            ("0.175", 18),
            ("0.25", 15),
            ("0.375", 10),
            ("0.50", 5),
            ("0.625", 3),
            ("0.75", 0),
            ("1", 0),
        ]

        for cv, expected in cases:
            with self.subTest(cv=cv):
                points = self.components(
                    revenue_cv=Decimal(cv),
                ).stability_points
                self.assertEqual(points, expected)

    def test_growth_scale(self):
        cases = [
            ("-0.5", 0),
            ("-0.20", 0),
            ("-0.125", 3),
            ("-0.05", 5),
            ("-0.025", 8),
            ("0", 10),
            ("0.05", 13),
            ("0.10", 15),
            ("0.20", 20),
            ("0.5", 20),
        ]

        for growth, expected in cases:
            with self.subTest(growth=growth):
                points = self.components(
                    revenue_growth_ratio=Decimal(growth),
                ).growth_points
                self.assertEqual(points, expected)

    def test_no_jumps_at_old_boundaries(self):
        # Раньше на границах диапазонов балл прыгал на 5–15 пунктов,
        # теперь соседние значения отличаются не больше чем на 1 балл.
        boundaries = {
            "debt_service_coverage_ratio": ["1.2", "1.5", "2"],
            "revenue_cv": ["0.10", "0.25", "0.50"],
            "revenue_growth_ratio": ["-0.20", "-0.05", "0.05"],
        }
        step = Decimal("0.0001")

        for field, values in boundaries.items():
            for value in values:
                with self.subTest(field=field, value=value):
                    left = calculate_score(
                        replace(self.base, **{field: Decimal(value) - step})
                    ).raw_score
                    right = calculate_score(
                        replace(self.base, **{field: Decimal(value) + step})
                    ).raw_score
                    self.assertLessEqual(abs(left - right), 1)

    def test_scale_is_monotonic(self):
        previous = None
        for i in range(100, 301):
            points = self.components(
                debt_service_coverage_ratio=Decimal(i) / 100,
            ).coverage_points
            if previous is not None:
                self.assertGreaterEqual(points, previous)
            previous = points

    def test_insufficient_coverage_caps_score(self):
        # Без ограничения получилось бы 0 + 20 + 20 = 40.
        metrics = replace(
            self.base,
            debt_service_coverage_ratio=Decimal("0.9"),
            revenue_cv=Decimal("0"),
            revenue_growth_ratio=Decimal("0.20"),
        )

        response = calculate_score(metrics)

        self.assertEqual(response.raw_score, 40)
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
