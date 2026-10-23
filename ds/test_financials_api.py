"""Тесты HTTP-контракта парсера; тяжёлый Docling заменяется mock.

Из папки ds: python -m unittest -v test_financials_api test_api
"""

import unittest
from unittest.mock import patch

from fastapi.testclient import TestClient

from api import app
from flexible_pdf_parser import FlexiblePdfError
from pdf_parser import PdfParsingError
from schemas import FinancialExtractionResponse, MonthlyFinancials


class FinancialsApiTests(unittest.TestCase):
    def setUp(self):
        self.client = TestClient(app)

    def tearDown(self):
        self.client.close()

    @staticmethod
    def expected():
        return FinancialExtractionResponse(
            inn="7700000016",
            financials=[
                MonthlyFinancials(
                    period_month=f"2026-{month:02d}",
                    revenue_kopecks=100_000_00,
                    operating_inflow_kopecks=90_000_00,
                    operating_outflow_kopecks=50_000_00,
                    tax_outflow_kopecks=10_000_00,
                )
                for month in range(4, 10)
            ],
            declared_monthly_debt_payment_kopecks=5_000_00,
        )

    def upload(self, content=b"%PDF-1.4\nexample", *, content_type="application/pdf"):
        return self.client.post(
            "/api/v1/financials/extract",
            files={"file": ("test.pdf", content, content_type)},
        )

    def test_fast_parser_success_and_response_contract(self):
        with patch("financials_api.parse_financial_pdf", return_value=self.expected()) as fast, \
             patch("financials_api.parse_flexible_pdf") as flexible:
            response = self.upload()
        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.json(), self.expected().model_dump(mode="json"))
        fast.assert_called_once()
        flexible.assert_not_called()

    def test_alternative_table_uses_docling(self):
        with patch("financials_api.parse_financial_pdf", side_effect=PdfParsingError(
            "Не найдена ровно одна финансовая таблица нужного формата"
        )), patch("financials_api.parse_flexible_pdf", return_value=self.expected()) as flexible:
            response = self.upload()
        self.assertEqual(response.status_code, 200)
        flexible.assert_called_once()

    def test_invalid_standard_data_not_masked_by_docling(self):
        with patch("financials_api.parse_financial_pdf", side_effect=PdfParsingError(
            "Некорректная сумма"
        )), patch("financials_api.parse_flexible_pdf") as flexible:
            response = self.upload()
        self.assertEqual(response.status_code, 422)
        self.assertEqual(response.json()["error"]["code"], "PDF_EXTRACTION_ERROR")
        flexible.assert_not_called()

    def test_broken_alternative_document_rejected(self):
        with patch("financials_api.parse_financial_pdf", side_effect=PdfParsingError(
            "Не найдена ровно одна финансовая таблица нужного формата"
        )), patch("financials_api.parse_flexible_pdf", side_effect=FlexiblePdfError(
            "Отсутствуют финансовые показатели"
        )):
            response = self.upload()
        self.assertEqual(response.status_code, 422)
        self.assertIn("Отсутствуют", response.json()["error"]["message"])

    def test_docling_missing(self):
        with patch("financials_api.parse_financial_pdf", side_effect=PdfParsingError(
            "Не найдена ровно одна финансовая таблица нужного формата"
        )), patch("financials_api.parse_flexible_pdf", side_effect=FlexiblePdfError(
            "Не установлен Docling — запусти из .venv-docling"
        )):
            response = self.upload()
        self.assertEqual(response.status_code, 503)
        self.assertEqual(response.json()["error"]["code"], "EXTRACTION_SERVICE_UNAVAILABLE")

    def test_empty_file(self):
        response = self.upload(b"")
        self.assertEqual(response.status_code, 400)
        self.assertEqual(response.json()["error"]["code"], "EMPTY_FILE")

    def test_wrong_file_signature(self):
        response = self.upload(b"text not pdf")
        self.assertEqual(response.status_code, 415)

    def test_wrong_mime(self):
        response = self.upload(content_type="text/plain")
        self.assertEqual(response.status_code, 415)

    def test_oversized_file(self):
        response = self.upload(b"%PDF-" + b"x" * (10 * 1024 * 1024))
        self.assertEqual(response.status_code, 413)

    def test_missing_upload(self):
        response = self.client.post("/api/v1/financials/extract")
        self.assertEqual(response.status_code, 422)
        self.assertEqual(response.json()["error"]["code"], "VALIDATION_ERROR")

    def test_both_endpoints_appear_in_swagger(self):
        response = self.client.get("/openapi.json")
        self.assertEqual(response.status_code, 200)
        paths = response.json()["paths"]
        self.assertIn("/api/v1/scoring/evaluate", paths)
        self.assertIn("/api/v1/financials/extract", paths)


if __name__ == "__main__":
    unittest.main(verbosity=2)
