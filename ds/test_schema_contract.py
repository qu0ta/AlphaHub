from datetime import date
import unittest

from pydantic import ValidationError

from schemas import ScoringInput, get_last_six_months
from metrics import calculate_financial_metrics
from test_metrics import make_input


class ContractTests(unittest.TestCase):
    def test_required_fields(self):
        for field in ('application_date', 'annual_rate_bps'):
            with self.subTest(field=field):
                payload = make_input().model_dump()
                del payload[field]
                with self.assertRaises(ValidationError):
                    ScoringInput.model_validate(payload)

    def test_old_application_is_valid(self):
        payload = make_input().model_dump()
        payload['application_date'] = '2025-01-15'
        for record, month in zip(payload['financials'], get_last_six_months(date(2025, 1, 15))):
            record['period_month'] = month
        result = ScoringInput.model_validate(payload)
        self.assertEqual(result.financials[0].period_month, '2024-07')

    def test_period_must_match_application_date(self):
        payload = make_input().model_dump()
        payload['application_date'] = '2026-11-01'
        with self.assertRaises(ValidationError):
            ScoringInput.model_validate(payload)

    def test_invalid_rates(self):
        for rate in (-1, 19.9, True):
            with self.subTest(rate=rate):
                payload = make_input().model_dump()
                payload['annual_rate_bps'] = rate
                with self.assertRaises(ValidationError):
                    ScoringInput.model_validate(payload)

    def test_rate_is_used_from_input(self):
        payload = make_input().model_dump()
        payload.update(amount_kopecks=100000, term_months=1, annual_rate_bps=1200)
        data = ScoringInput.model_validate(payload)
        result = calculate_financial_metrics(data)
        self.assertEqual(result.new_monthly_payment_kopecks, 101000)


if __name__ == '__main__':
    unittest.main()
