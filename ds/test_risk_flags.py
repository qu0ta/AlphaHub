import unittest

from metrics import calculate_financial_metrics
from schemas import ScoringInput
from scoring import score_application
from test_metrics import make_input


class RiskFlagTests(unittest.TestCase):
    def test_decline_boundary(self):
        for recent, expected in [
            (81, False),  # -19%
            (80, False),  # -20%
            (79, True),   # -21%
        ]:
            with self.subTest(recent=recent):
                data = make_input([100] * 3 + [recent] * 3)
                flags = calculate_financial_metrics(data).flags

                self.assertEqual(
                    "REVENUE_DECLINE_GT_20_PERCENT" in flags,
                    expected,
                )

    def test_zero_base_is_not_decline(self):
        data = make_input([0] * 3 + [100] * 3)
        flags = calculate_financial_metrics(data).flags

        self.assertNotIn("REVENUE_DECLINE_GT_20_PERCENT", flags)

    def test_zero_revenue_with_inflow(self):
        data = make_input([0] * 6)
        flags = calculate_financial_metrics(data).flags

        self.assertEqual(
            set(flags),
            {
                "ZERO_REVENUE",
                "ZERO_REVENUE_BASE",
                "ZERO_REVENUE_WITH_INFLOW",
            },
        )
        self.assertEqual(len(flags), len(set(flags)))

        # Новый признак не меняет финансовый балл.
        self.assertEqual(score_application(data).score, 60)

    def test_zero_revenue_without_inflow(self):
        payload = make_input([0] * 6).model_dump()

        for month in payload["financials"]:
            month["operating_inflow_kopecks"] = 0

        data = ScoringInput.model_validate(payload)
        flags = calculate_financial_metrics(data).flags

        self.assertNotIn("ZERO_REVENUE_WITH_INFLOW", flags)
        self.assertIn("ZERO_REVENUE", flags)

    def test_positive_revenue(self):
        flags = calculate_financial_metrics(make_input()).flags

        self.assertNotIn("ZERO_REVENUE_WITH_INFLOW", flags)

    def test_high_score_and_contract_preserved(self):
        data = make_input([100] * 3 + [79] * 3)
        result = score_application(data)

        # Падение на 21% может сочетаться с высоким score.
        self.assertEqual(result.score, 75)

        self.assertEqual(
            set(result.model_dump()),
            {"score", "explanation", "model_version"},
        )


if __name__ == "__main__":
    unittest.main()