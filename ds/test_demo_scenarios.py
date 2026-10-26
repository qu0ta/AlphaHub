"""Проверка демо-ожиданий и опубликованных примеров через настоящий DS API."""

import json
from pathlib import Path
import re
import runpy
import unittest

from fastapi.testclient import TestClient

from api import app


ROOT = Path(__file__).resolve().parents[1]
DEMO = runpy.run_path(str(ROOT / "scripts" / "smoke_demo.py"))


class DemoScenariosTests(unittest.TestCase):
    def setUp(self):
        self.client = TestClient(app)
        self.addCleanup(self.client.close)

    def evaluate(self, payload):
        response = self.client.post("/api/v1/scoring/evaluate", json=payload)
        self.assertEqual(response.status_code, 200, response.text)
        return response.json()

    def test_demo_pdfs_cover_all_three_routes(self):
        self.assertEqual(DEMO["MODEL_VERSION"], "financial_rules_v2")
        self.assertEqual(
            {profile[5] for profile in DEMO["DEMO_PROFILES"].values()},
            {"OFFERED", "MANUAL", "REJECTED"},
        )
        for name, (_, inn, amount, term, score, status) in DEMO["DEMO_PROFILES"].items():
            with self.subTest(profile=name):
                pdf = ROOT / "ds" / "demo_pdfs" / f"{name}.pdf"
                extraction = self.client.post(
                    "/api/v1/financials/extract",
                    files={"file": (pdf.name, pdf.read_bytes(), "application/pdf")},
                )
                self.assertEqual(extraction.status_code, 200, extraction.text)
                self.assertEqual(extraction.json()["inn"], inn)
                result = self.evaluate({
                    **extraction.json(),
                    "amount_kopecks": amount,
                    "term_months": term,
                    "annual_rate_bps": 1990,
                    "application_date": DEMO["DEMO_MONTH"] + "-01",
                })
                self.assertEqual(result["model_version"], DEMO["MODEL_VERSION"])
                self.assertEqual(result["score"], score)
                route = "REJECTED" if score < 40 else "MANUAL" if score < 70 else "OFFERED"
                self.assertEqual(route, status)

    def test_json_contract_example_matches_api(self):
        examples = json.loads((ROOT / "docs" / "ds" / "examples.json").read_text(encoding="utf-8"))
        self.assertEqual(self.evaluate(examples["ScoringInput"]), examples["ScoringResponse"])

    def test_openapi_example_matches_api(self):
        contract = json.loads((ROOT / "docs" / "ds" / "openapi.json").read_text(encoding="utf-8"))
        operation = contract["paths"]["/api/v1/scoring/evaluate"]["post"]
        payload = operation["requestBody"]["content"]["application/json"]["example"]
        expected = operation["responses"]["200"]["content"]["application/json"]["example"]
        self.assertEqual(self.evaluate(payload), expected)

    def test_markdown_examples_match_api(self):
        examples = json.loads((ROOT / "docs" / "ds" / "examples.json").read_text(encoding="utf-8"))
        contract = (ROOT / "docs" / "ds" / "CONTRACT.md").read_text(encoding="utf-8")
        responses = [json.loads(block) for block in re.findall(r"~~~json\n(.*?)\n~~~", contract, re.S)]
        expected = next(block for block in responses if "score" in block)
        self.assertEqual(self.evaluate(examples["ScoringInput"]), expected)

        contract = (ROOT / "docs" / "API_CONTRACTS.md").read_text(encoding="utf-8")
        section = contract.split("### 5.23.", 1)[1].split("## 6.", 1)[0]
        payload, expected = [json.loads(block) for block in re.findall(r"```json\n(.*?)\n```", section, re.S)]
        self.assertEqual(self.evaluate(payload), expected)


if __name__ == "__main__":
    unittest.main()
