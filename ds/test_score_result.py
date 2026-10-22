import unittest
from dataclasses import replace
from decimal import Decimal

from metrics import calculate_financial_metrics
from schemas import ScoringInput
from scoring import (
    calculate_score,
    calculate_score_from_metrics,
    score_application,
)
from test_metrics import make_input


class ScoreResultTests(unittest.TestCase):
    def test_components(self):
        result = calculate_score(make_input())

        self.assertEqual(
            result.components.model_dump(),
            {
                "coverage_points": 60,
                "stability_points": 20,
                "growth_points": 10,
            },
        )
        self.assertEqual(result.raw_score, 90)
        self.assertEqual(result.score, 90)
        self.assertIsNone(result.score_cap)

    def test_cap_is_explained(self):
        metrics = replace(
            calculate_financial_metrics(make_input()),
            debt_service_coverage_ratio=Decimal("0.9"),
            revenue_cv=Decimal("0.08"),
            revenue_growth_ratio=Decimal("0.10"),
        )

        result = calculate_score_from_metrics(metrics)

        self.assertEqual(result.raw_score, 40)
        self.assertEqual(result.score_cap, 39)
        self.assertEqual(result.score, 39)

    def test_deterministic_and_no_input_mutation(self):
        data = make_input()
        before = data.model_dump_json()

        first = calculate_score(data).model_dump_json()

        restored_data = ScoringInput.model_validate_json(before)
        second = calculate_score(restored_data).model_dump_json()

        self.assertEqual(first, second)
        self.assertEqual(data.model_dump_json(), before)

    def test_backend_contract(self):
        result = score_application(make_input())

        self.assertEqual(
            set(result.model_dump()),
            {"score", "explanation", "model_version"},
        )


if __name__ == "__main__":
    unittest.main()