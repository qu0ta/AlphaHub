"""Запуск из ds: ../.venv-docling/bin/python test_flexible_pdf_parser.py"""
import csv
import unittest
from pathlib import Path

from flexible_pdf_parser import FlexiblePdfError, parse_docling_data, parse_docling_exports

BASE = Path(__file__).resolve().parent

# Для локального запуска тестов с артефактами здесь файлы расположены рядом.
MARKDOWN = BASE / "docling_output" / "stable_p1-1.md"
CSV = BASE / "docling_output" / "stable_p1-1_table_1.csv"


class FlexibleParserTests(unittest.TestCase):
    def setUp(self):
        self.md = MARKDOWN.read_text(encoding="utf-8")
        with CSV.open(encoding="utf-8-sig", newline="") as fp:
            self.rows = list(csv.DictReader(fp))

    def test_docling_export(self):
        result = parse_docling_exports(MARKDOWN, [CSV])
        self.assertEqual(result.inn, "7700000016")
        self.assertEqual(len(result.financials), 6)
        self.assertEqual(result.financials[0].revenue_kopecks, 91334866)
        self.assertEqual(result.financials[1].operating_inflow_kopecks, 100060798)
        self.assertEqual(result.declared_monthly_debt_payment_kopecks, 3177139)

    def test_renamed_and_reordered_columns(self):
        aliases = {
            "Месяц": "Отчётный месяц",
            "Выручка": "Выручка от продаж",
            "Поступления": "Поступления от основной деятельности",
            "Операционные выплаты": "Операционные платежи",
            "Налоги": "Уплаченные налоги",
        }
        renamed_rows = [{aliases[k]: v for k, v in reversed(list(row.items()))} for row in self.rows]
        actual = parse_docling_data(self.md, [renamed_rows])
        self.assertEqual(actual.financials[-1].tax_outflow_kopecks, 6627528)

    def test_five_months_rejected(self):
        with self.assertRaisesRegex(FlexiblePdfError, "шесть месяцев"):
            parse_docling_data(self.md, [self.rows[:-1]])

    def test_missing_column_rejected(self):
        rows = [{key: value for key, value in row.items() if key != "Налоги"} for row in self.rows]
        with self.assertRaisesRegex(FlexiblePdfError, "помесячная финансовая таблица"):
            parse_docling_data(self.md, [rows])

    def test_missing_debt_rejected(self):
        md = "\n".join(x for x in self.md.splitlines() if "Ежемесячный платёж" not in x)
        with self.assertRaisesRegex(FlexiblePdfError, "ежемесячный"):
            parse_docling_data(md, [self.rows])

    def test_wrong_amount_rejected(self):
        rows = [dict(x) for x in self.rows]
        rows[0]["Выручка"] = "неизвестно"
        with self.assertRaisesRegex(FlexiblePdfError, "Некорректное денежное"):
            parse_docling_data(self.md, [rows])

    def test_missing_month_rejected(self):
        rows = [dict(x) for x in self.rows]
        rows[3]["Месяц"] = "2026-08"
        with self.assertRaisesRegex(FlexiblePdfError, "Месяцы должны идти подряд"):
            parse_docling_data(self.md, [rows])

    def transposed_rows(self):
        """Формат Docling: Показатель, 2026-04, ..., 2026-09."""
        months = [row["Месяц"] for row in self.rows]
        aliases = {
            "Выручка": "Выручка от продаж",
            "Поступления": "Поступления от основной деятельности",
            "Операционные выплаты": "Операционные платежи",
            "Налоги": "Уплаченные налоги",
        }
        return [
            {"Показатель": aliases[field], **{
                month: row[field] for month, row in zip(months, self.rows)
            }}
            for field in aliases
        ]

    def test_transposed_format_matches_original(self):
        actual = parse_docling_data(self.md, [self.transposed_rows()])
        original = parse_docling_data(self.md, [self.rows])
        self.assertEqual(actual, original)

    def test_transposed_metric_column_alias(self):
        rows = self.transposed_rows()
        for row in rows:
            row["Наименование показателя"] = row.pop("Показатель")
        actual = parse_docling_data(self.md, [rows])
        self.assertEqual(actual.financials[0].revenue_kopecks, 91334866)

    def test_transposed_missing_metric_rejected(self):
        with self.assertRaisesRegex(FlexiblePdfError, "Отсутствуют финансовые показатели"):
            parse_docling_data(self.md, [self.transposed_rows()[:-1]])

    def test_transposed_duplicate_metric_rejected(self):
        rows = self.transposed_rows()
        rows.append(dict(rows[0]))
        with self.assertRaisesRegex(FlexiblePdfError, "Повторяется финансовый показатель"):
            parse_docling_data(self.md, [rows])

    def test_transposed_five_month_headers_rejected(self):
        rows = self.transposed_rows()
        for row in rows:
            row.pop("2026-09")
        with self.assertRaisesRegex(FlexiblePdfError, "ровно шесть месяцев"):
            parse_docling_data(self.md, [rows])

    def test_transposed_months_out_of_order_rejected(self):
        rows = self.transposed_rows()
        # Имитируем PDF, где сентябрь напечатан перед августом.
        rows = [
            {"Показатель": row["Показатель"],
             **{month: row[month] for month in
                ["2026-04", "2026-05", "2026-06", "2026-07", "2026-09", "2026-08"]}}
            for row in rows
        ]
        with self.assertRaisesRegex(FlexiblePdfError, "Месяцы должны идти подряд"):
            parse_docling_data(self.md, [rows])

    def test_transposed_wrong_amount_rejected(self):
        rows = self.transposed_rows()
        rows[1]["2026-05"] = "неизвестно"
        with self.assertRaisesRegex(FlexiblePdfError, "Некорректное денежное"):
            parse_docling_data(self.md, [rows])

    def test_transposed_extra_metric_allowed(self):
        rows = self.transposed_rows()
        rows.append({"Показатель": "Примечания к отчётности"})
        actual = parse_docling_data(self.md, [rows])
        self.assertEqual(len(actual.financials), 6)

    def test_both_orientations_in_one_pdf_rejected(self):
        with self.assertRaisesRegex(FlexiblePdfError, "ровно одна помесячная"):
            parse_docling_data(self.md, [self.rows, self.transposed_rows()])


if __name__ == "__main__":
    unittest.main(verbosity=2)
