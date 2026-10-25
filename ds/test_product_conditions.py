import unittest

from metrics import calculate_financial_metrics
from schemas import ScoringInput
from scoring import score_application
from test_metrics import make_input


def equipment_input(
    rate_bps: int = 1800,
    term_months: int = 24,
) -> ScoringInput:
    payload = make_input().model_dump(mode="json")

    payload.update({
        "amount_kopecks": 2_000_000 * 100,
        "term_months": term_months,
        "annual_rate_bps": rate_bps,
    })

    return ScoringInput.model_validate(payload)


class ProductConditionsTests(unittest.TestCase):
    def test_equipment_product_can_be_scored(self):
        result = score_application(equipment_input())

        self.assertGreaterEqual(result.score, 0)
        self.assertLessEqual(result.score, 100)
        self.assertEqual(
            set(result.model_dump()),
            {"score", "explanation", "model_version"},
        )

    def test_higher_rate_increases_payment(self):
        lower = equipment_input(rate_bps=1800)
        higher = equipment_input(rate_bps=2400)

        lower_metrics = calculate_financial_metrics(lower)
        higher_metrics = calculate_financial_metrics(higher)

        self.assertGreater(
            higher_metrics.new_monthly_payment_kopecks,
            lower_metrics.new_monthly_payment_kopecks,
        )
        self.assertLess(
            higher_metrics.debt_service_coverage_ratio,
            lower_metrics.debt_service_coverage_ratio,
        )
        self.assertLessEqual(
            score_application(higher).score,
            score_application(lower).score,
        )

    def test_longer_term_reduces_monthly_payment(self):
        shorter = equipment_input(term_months=12)
        longer = equipment_input(term_months=36)

        shorter_metrics = calculate_financial_metrics(shorter)
        longer_metrics = calculate_financial_metrics(longer)

        self.assertLess(
            longer_metrics.new_monthly_payment_kopecks,
            shorter_metrics.new_monthly_payment_kopecks,
        )
        self.assertGreater(
            longer_metrics.debt_service_coverage_ratio,
            shorter_metrics.debt_service_coverage_ratio,
        )
        self.assertGreaterEqual(
            score_application(longer).score,
            score_application(shorter).score,
        )


if __name__ == "__main__":
    unittest.main()