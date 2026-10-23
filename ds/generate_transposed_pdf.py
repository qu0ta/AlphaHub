"""Генератор альтернативного PDF для проверки гибкого парсера.

Запуск из AlphaHub/ds:
    ../.venv/bin/python generate_transposed_pdf.py

Использует данные из fixtures/stable.json и функции из pdf_generator.py.
"""
from pathlib import Path
from xml.sax.saxutils import escape

from reportlab.lib import colors
from reportlab.lib.pagesizes import A4, landscape
from reportlab.lib.styles import ParagraphStyle
from reportlab.platypus import SimpleDocTemplate, Paragraph, Spacer, Table, TableStyle

from pdf_generator import FONT_NAME, format_rubles, register_font
from schemas import ScoringInput

BASE = Path(__file__).resolve().parent


def make_transposed_pdf():
    source = BASE / "fixtures" / "stable.json"
    data = ScoringInput.model_validate_json(source.read_text(encoding="utf-8"))
    register_font()

    out = BASE / "demo_pdfs" / "stable_transposed.pdf"
    out.parent.mkdir(exist_ok=True, parents=True)

    doc = SimpleDocTemplate(
        str(out), pagesize=landscape(A4),
        leftMargin=29, rightMargin=29, topMargin=37, bottomMargin=35,
    )
    title = ParagraphStyle("AlternateTitle", fontName=FONT_NAME, fontSize=16, leading=22)
    regular = ParagraphStyle("AlternateText", fontName=FONT_NAME, fontSize=10, leading=15)
    header = ParagraphStyle("AlternateHeader", fontName=FONT_NAME, fontSize=8, leading=12)

    story = [
        Paragraph("Финансовая сводка компании — альтернативная форма", title),
        Spacer(1, 12),
        Paragraph("Компания: ООО «Северный Маяк»", regular),
        Paragraph("ИНН: " + escape(data.inn), regular),
        Spacer(1, 16),
        Paragraph("Финансовые показатели (суммы в рублях)", regular),
        Spacer(1, 9),
    ]
    months = [r.period_month for r in data.financials]
    table_rows = [[Paragraph("Показатель", header), *months]]

    metrics = [
        ("Выручка от продаж", "revenue_kopecks"),
        ("Поступления от основной деятельности", "operating_inflow_kopecks"),
        ("Операционные платежи", "operating_outflow_kopecks"),
        ("Уплаченные налоги", "tax_outflow_kopecks"),
    ]
    for label, field in metrics:
        table_rows.append([
            Paragraph(escape(label), header),
            *(format_rubles(getattr(r, field)) for r in data.financials),
        ])

    table = Table(table_rows, colWidths=[190] + [96] * 6, rowHeights=[35] + [43] * 4)
    table.setStyle(TableStyle([
        ("FONTNAME", (0, 0), (-1, -1), FONT_NAME),
        ("FONTSIZE", (1, 0), (-1, -1), 8.2),
        ("BACKGROUND", (0, 0), (-1, 0), colors.HexColor("#E8EEF4")),
        ("GRID", (0, 0), (-1, -1), 0.45, colors.HexColor("#BAC7D1")),
        ("VALIGN", (0, 0), (-1, -1), "MIDDLE"),
        ("ALIGN", (1, 0), (-1, -1), "RIGHT"),
        ("LEFTPADDING", (0, 0), (-1, -1), 8),
        ("RIGHTPADDING", (0, 0), (-1, -1), 8),
    ]))
    story += [
        table,
        Spacer(1, 19),
        Paragraph(
            "Ежемесячный платёж по действующим кредитам: "
            + format_rubles(data.declared_monthly_debt_payment_kopecks)
            + " руб.", regular,
        ),
        Spacer(1, 10),
        Paragraph("Синтетический тестовый документ: альтернативное расположение показателей.", header),
    ]
    doc.build(story)
    print("Создан:", out)


if __name__ == "__main__":
    make_transposed_pdf()
