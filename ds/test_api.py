import unittest
from unittest.mock import patch

from fastapi.testclient import TestClient

from api import app
from scoring import score_application
from test_metrics import make_input


class ScoringAPITests(unittest.TestCase):
    def setUp(self):
        self.client = TestClient(app)

    def tearDown(self):
        self.client.close()

    def payload(self):
        return make_input().model_dump(mode="json")

    def test_success_matches_direct_calculation(self):
        response = self.client.post(
            "/api/v1/scoring/evaluate",
            json=self.payload(),
        )

        self.assertEqual(response.status_code, 200)

        expected = score_application(make_input()).model_dump(mode="json")

        self.assertEqual(response.json(), expected)
        self.assertEqual(
            set(response.json()),
            {"score", "explanation", "model_version"},
        )

    def test_deterministic_response(self):
        payload = self.payload()

        first = self.client.post("/api/v1/scoring/evaluate", json=payload)
        second = self.client.post("/api/v1/scoring/evaluate", json=payload)

        self.assertEqual(first.status_code, 200)
        self.assertEqual(second.status_code, 200)
        self.assertEqual(first.json(), second.json())

    def test_invalid_input_does_not_call_scoring(self):
        for case in ["missing_rate", "negative_amount", "wrong_month"]:
            with self.subTest(case=case):
                payload = self.payload()

                if case == "missing_rate":
                    del payload["annual_rate_bps"]
                elif case == "negative_amount":
                    payload["amount_kopecks"] = -1
                else:
                    payload["financials"][0]["period_month"] = "2020-01"

                with patch("api.score_application") as scoring:
                    response = self.client.post(
                        "/api/v1/scoring/evaluate",
                        json=payload,
                    )

                self.assertEqual(response.status_code, 422)
                self.assertIn("error", response.json())
                self.assertNotIn("detail", response.json())
                self.assertEqual(
                    response.json()["error"]["code"],
                    "VALIDATION_ERROR",
                )
                self.assertTrue(response.json()["error"]["message"])    
                self.assertNotIn("score", response.json())
                scoring.assert_not_called()

    def test_invalid_json(self):
        response = self.client.post(
            "/api/v1/scoring/evaluate",
            content="{invalid json",
            headers={"Content-Type": "application/json"},
        )

        self.assertEqual(response.status_code, 422)
        self.assertEqual(
            response.json()["error"]["code"],
            "VALIDATION_ERROR",
        )
        self.assertNotIn("detail", response.json())
        
    def test_calculation_error(self):
        with patch(
            "api.score_application",
            side_effect=ValueError("Undefined coverage"),
        ):
            response = self.client.post(
                "/api/v1/scoring/evaluate",
                json=self.payload(),
            )

        self.assertEqual(response.status_code, 422)
        self.assertEqual(
            response.json()["error"]["code"],
            "SCORING_NOT_CALCULABLE",
        )
        self.assertNotIn("score", response.json())

    def test_unexpected_error(self):
        with patch(
            "api.score_application",
            side_effect=RuntimeError("Internal diagnostic information"),
        ):
            with self.assertLogs("api", level="ERROR"):
                response = self.client.post(
                    "/api/v1/scoring/evaluate",
                    json=self.payload(),
                )

        self.assertEqual(response.status_code, 500)
        self.assertEqual(
            response.json()["error"]["code"],
            "INTERNAL_ERROR",
        )
        self.assertNotIn(
            "Internal diagnostic information",
            response.text,
        )

    def test_swagger_and_openapi_available(self):
        self.assertEqual(self.client.get("/docs").status_code, 200)

        response = self.client.get("/openapi.json")

        self.assertEqual(response.status_code, 200)
        self.assertIn(
            "/api/v1/scoring/evaluate",
            response.json()["paths"],
        )


if __name__ == "__main__":
    unittest.main()