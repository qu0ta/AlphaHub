"""DS-08: негативные тесты PDF-парсера. Запуск: python test_pdf_validation.py"""

import tempfile
import unittest
from pathlib import Path

from reportlab.lib import colors
from reportlab.lib.pagesizes import A4
from reportlab.lib.styles import ParagraphStyle
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.pdfgen.canvas import Canvas
from reportlab.platypus import Paragraph, SimpleDocTemplate, Spacer, Table, TableStyle

from pdf_parser import parse_financial_pdf, money_to_kopecks


MONTHS = ["2026-04", "2026-05", "2026-06", "2026-07", "2026-08", "2026-09"]
HEADERS = ["Месяц", "Выручка", "Поступления", "Операционные выплаты", "Налоги"]
DEFAULT_ROW = ["1 000 000,00", "900 000,00", "600 000,00", "100 000,00"]
FONT = "DS08Cyrillic"


def register_test_font():
    if FONT in pdfmetrics.getRegisteredFontNames():
        return
    candidates = [
        Path(__file__).resolve().parent / "fonts" / "DejaVuSans.ttf",
        Path("/System/Library/Fonts/Supplemental/Arial.ttf"),
        Path("/Library/Fonts/Arial.ttf"),
        Path("C:/Windows/Fonts/arial.ttf"),
        Path("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"),
    ]
    for path in candidates:
        if path.is_file():
            pdfmetrics.registerFont(TTFont(FONT, str(path)))
            return
    raise RuntimeError("Не найден TTF-шрифт с кириллицей для тестов")


def make_financial_pdf(
    path,
    *,
    months=None,
    headers=None,
    inn="7700000016",
    debt="50 000,00",
    include_inn=True,
    include_debt=True,
    duplicate_inn=False,
    duplicate_debt=False,
    bad_amount=None,
):
    """Создать PDF с заданной ошибкой, не меняя файлы demo_pdfs/."""
    months = MONTHS if months is None else months
    headers = HEADERS if headers is None else headers
    style = ParagraphStyle("Test", fontName=FONT, fontSize=9, leading=14)
    small = ParagraphStyle("Small", fontName=FONT, fontSize=7.5, leading=10)
    doc = SimpleDocTemplate(
        str(path), pagesize=A4, leftMargin=34, rightMargin=34,
        topMargin=30, bottomMargin=30,
    )
    story = [Paragraph("Финансовая сводка компании", style)]
    if include_inn:
        story.append(Paragraph("ИНН: " + inn, style))
        if duplicate_inn:
            story.append(Paragraph("ИНН: " + inn, style))
    story.append(Spacer(1, 12))
    rows = [[Paragraph(value, small) for value in headers]]
    for idx, month in enumerate(months):
        amount = DEFAULT_ROW.copy()
        if bad_amount is not None and idx == 0:
            amount[0] = bad_amount
        rows.append([month] + amount)
    table = Table(rows, colWidths=[68, 110, 110, 137, 100], rowHeights=[40] + [32] * len(months))
    table.setStyle(TableStyle([
        ("FONTNAME", (0, 0), (-1, -1), FONT),
        ("FONTSIZE", (0, 1), (-1, -1), 8),
        ("GRID", (0, 0), (-1, -1), 0.5, colors.grey),
        ("VALIGN", (0, 0), (-1, -1), "MIDDLE"),
    ]))
    story.extend([table, Spacer(1, 16)])
    if include_debt:
        text = "Ежемесячный платёж по действующим кредитам: " + debt + " руб."
        story.append(Paragraph(text, style))
        if duplicate_debt:
            story.append(Paragraph(text, style))
    doc.build(story)


class PdfValidationTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        register_test_font()

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.path = Path(self.tmp.name) / "test.pdf"

    def expect_error(self, message, **kwargs):
        make_financial_pdf(self.path, **kwargs)
        with self.assertRaisesRegex(ValueError, message):
            parse_financial_pdf(self.path)

    def test_valid_document_still_works(self):
        make_financial_pdf(self.path)
        result = parse_financial_pdf(self.path)
        self.assertEqual(result.inn, "7700000016")
        self.assertEqual(len(result.financials), 6)
        self.assertEqual(result.financials[0].revenue_kopecks, 100000000)
        self.assertEqual(result.declared_monthly_debt_payment_kopecks, 5000000)

    def test_missing_file(self):
        with self.assertRaisesRegex(ValueError, "не найден"):
            parse_financial_pdf(self.path)

    def test_empty_file(self):
        self.path.write_bytes(b"")
        with self.assertRaisesRegex(ValueError, "пуст"):
            parse_financial_pdf(self.path)

    def test_wrong_file_type(self):
        self.path.write_text("Это не PDF", encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "не является PDF"):
            parse_financial_pdf(self.path)

    def test_corrupted_pdf(self):
        self.path.write_bytes(b"%PDF-1.7\nbroken content")
        with self.assertRaisesRegex(ValueError, "Не удалось прочитать PDF"):
            parse_financial_pdf(self.path)

    def test_larger_than_10_mb(self):
        with self.path.open("wb") as file:
            file.truncate(10 * 1024 * 1024 + 1)
        with self.assertRaisesRegex(ValueError, "10 МБ"):
            parse_financial_pdf(self.path)

    def test_pdf_without_text(self):
        pdf = Canvas(str(self.path), pagesize=A4)
        pdf.showPage()
        pdf.save()
        with self.assertRaisesRegex(ValueError, "нет извлекаемого текста"):
            parse_financial_pdf(self.path)

    def test_missing_inn(self):
        self.expect_error("ИНН", include_inn=False)

    def test_invalid_inn(self):
        self.expect_error("ИНН", inn="123")

    def test_duplicate_inn(self):
        self.expect_error("ровно один ИНН", duplicate_inn=True)

    def test_missing_debt(self):
        self.expect_error("долговой платёж", include_debt=False)

    def test_duplicate_debt(self):
        self.expect_error("ровно один", duplicate_debt=True)

    def test_invalid_amount(self):
        self.expect_error("Некорректная сумма", bad_amount="12x,30")

    def test_negative_amount(self):
        self.expect_error("Некорректная сумма", bad_amount="-100,00")

    def test_missing_month(self):
        self.expect_error("ровно 6 месяцев", months=MONTHS[:-1])

    def test_month_gap(self):
        self.expect_error("Месяцы должны идти подряд", months=[
            "2026-04", "2026-05", "2026-06", "2026-08", "2026-09", "2026-10"
        ])

    def test_duplicate_month(self):
        self.expect_error("Месяцы должны идти подряд", months=[
            "2026-04", "2026-05", "2026-06", "2026-06", "2026-08", "2026-09"
        ])

    def test_wrong_column_header(self):
        self.expect_error("финансовая таблица", headers=[
            "Месяц", "Доходы", "Поступления", "Операционные выплаты", "Налоги"
        ])

    def test_malformed_debt(self):
        self.expect_error("долговой платёж", debt="50x,00")

    def test_money_precision(self):
        self.assertEqual(money_to_kopecks("913 348,66"), 91334866)
        self.assertEqual(money_to_kopecks("0,01"), 1)


if __name__ == "__main__":
    unittest.main(verbosity=2)
