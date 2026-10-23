
from decimal import Decimal
from pathlib import Path
from xml.sax.saxutils import escape

from reportlab.lib import colors
from reportlab.lib.pagesizes import A4
from reportlab.lib.styles import ParagraphStyle
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.platypus import (
    SimpleDocTemplate,
    Paragraph,
    Spacer,
    Table,
    TableStyle,
)

from schemas import ScoringInput


BASE_DIR = Path(__file__).resolve().parent
FONT_NAME = "FinanceCyrillic"


def register_font():
    if FONT_NAME in pdfmetrics.getRegisteredFontNames():
        return

    # Ищем шрифт с поддержкой русского текста
    candidates = [
        BASE_DIR / "fonts" / "DejaVuSans.ttf",
        Path("/System/Library/Fonts/Supplemental/Arial.ttf"),
        Path("/Library/Fonts/Arial.ttf"),
        Path("C:/Windows/Fonts/arial.ttf"),
        Path("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"),
    ]

    for font_path in candidates:
        if font_path.is_file():
            pdfmetrics.registerFont(
                TTFont(FONT_NAME, str(font_path))
            )
            return

    raise FileNotFoundError(
        "Не найден TTF-шрифт с кириллицей. "
        "Добавь DejaVuSans.ttf в папку ds/fonts/."
    )


def format_rubles(value_kopecks: int) -> str:
    rubles = Decimal(value_kopecks) / Decimal(100)

    return (
        f"{rubles:,.2f}"
        .replace(",", " ")
        .replace(".", ",")
    )


def create_financial_pdf(
    data: ScoringInput,
    company_name: str,
    output_path: Path,
) -> Path:

    register_font()

    output_path = Path(output_path)
    output_path.parent.mkdir(
        parents=True,
        exist_ok=True
    )

    # Стили текста
    title_style = ParagraphStyle(
        "FinanceTitle",
        fontName=FONT_NAME,
        fontSize=16,
        leading=22,
        spaceAfter=14,
    )

    text_style = ParagraphStyle(
        "FinanceText",
        fontName=FONT_NAME,
        fontSize=10,
        leading=15,
    )

    small_style = ParagraphStyle(
        "FinanceSmall",
        fontName=FONT_NAME,
        fontSize=8.5,
        leading=12,
    )

    header_style = ParagraphStyle(
        "FinanceHeader",
        fontName=FONT_NAME,
        fontSize=8,
        leading=11,
    )

    # Документ формата A4
    document = SimpleDocTemplate(
        str(output_path),
        pagesize=A4,
        leftMargin=33,
        rightMargin=33,
        topMargin=40,
        bottomMargin=40,
    )

    # Заголовок и информация о компании
    story = [
        Paragraph(
            "Финансовая сводка компании",
            title_style
        ),
        Paragraph(
            "Компания: " + escape(company_name),
            text_style
        ),
        Paragraph(
            "ИНН: " + escape(data.inn),
            text_style
        ),
        Paragraph(
            "Период: "
            + data.financials[0].period_month
            + " — "
            + data.financials[-1].period_month,
            text_style
        ),
        Spacer(1, 18),
        Paragraph(
            "Финансовые показатели (суммы в рублях)",
            text_style
        ),
        Spacer(1, 9),
    ]

    # Заголовки финансовой таблицы
    header = [
        Paragraph("Месяц", header_style),
        Paragraph("Выручка", header_style),
        Paragraph("Поступления", header_style),
        Paragraph("Операционные выплаты", header_style),
        Paragraph("Налоги", header_style),
    ]

    table_data = [header]

    # Шесть строк финансовых показателей
    for record in data.financials:
        table_data.append([
            record.period_month,
            format_rubles(record.revenue_kopecks),
            format_rubles(record.operating_inflow_kopecks),
            format_rubles(record.operating_outflow_kopecks),
            format_rubles(record.tax_outflow_kopecks),
        ])

    # Создаём таблицу
    table = Table(
        table_data,
        colWidths=[66, 108, 108, 143, 98],
        rowHeights=[39] + [34] * 6,
    )

    table.setStyle(TableStyle([
        ("FONTNAME", (0, 0), (-1, -1), FONT_NAME),
        ("FONTSIZE", (0, 1), (-1, -1), 8.7),

        ("BACKGROUND", (0, 0), (-1, 0),
         colors.HexColor("#E8EEF4")),

        ("GRID", (0, 0), (-1, -1),
         0.45, colors.HexColor("#CED5DF")),

        ("VALIGN", (0, 0), (-1, -1), "MIDDLE"),
        ("ALIGN", (1, 1), (-1, -1), "RIGHT"),

        ("LEFTPADDING", (0, 0), (-1, -1), 7),
        ("RIGHTPADDING", (0, 0), (-1, -1), 7),
    ]))

    story.append(table)

    # Долговой платёж
    story.extend([
        Spacer(1, 18),

        Paragraph(
            "Ежемесячный платёж по действующим кредитам: "
            + format_rubles(
                data.declared_monthly_debt_payment_kopecks
            )
            + " руб.",
            text_style
        ),

        Spacer(1, 18),

        Paragraph(
            "Синтетический документ для демонстрации "
            "и тестирования. Не является реальной "
            "финансовой отчётностью.",
            small_style
        ),
    ])

    document.build(story)

    return output_path


if __name__ == "__main__":

    company_names = {
        "stable": "ООО «Северный Маяк»",
        "medium": "ООО «Деловой Вектор»",
        "risky": "ООО «Городские Решения»",
    }

    for profile, company_name in company_names.items():

        json_path = (
            BASE_DIR / "fixtures" / (profile + ".json")
        )

        data = ScoringInput.model_validate_json(
            json_path.read_text(encoding="utf-8")
        )

        pdf_path = (
            BASE_DIR / "demo_pdfs" / (profile + ".pdf")
        )

        create_financial_pdf(
            data,
            company_name,
            pdf_path
        )

        print("Создан PDF:", pdf_path)
