from pathlib import Path
import re

import pdfplumber

from schemas import FinancialExtractionResponse, MonthlyFinancials


MAX_PDF_SIZE = 10 * 1024 * 1024
HEADERS = ["Месяц", "Выручка", "Поступления", "Операционные выплаты", "Налоги"]
DEBT_LABEL = "Ежемесячный платёж по действующим кредитам:"
MONEY_PATTERN = r"(?:\d{1,3}(?:[ \u00a0\u202f]\d{3})+|\d+),\d{2}"


class PdfParsingError(ValueError):
    """Документ невозможно обработать без догадок о финансовых значениях."""


def normalized(cell):
    return " ".join(str(cell or "").split())


def money_to_kopecks(value):
    value = normalized(value)
    if re.fullmatch(MONEY_PATTERN, value) is None:
        raise PdfParsingError(f"Некорректная сумма: {value!r}")
    rubles, kopecks = value.split(",")
    return int(rubles.replace(" ", "")) * 100 + int(kopecks)


def _unique_line(text, prefix, message):
    """Обязательный реквизит должен встретиться ровно один раз."""
    matches = [
        line.strip()
        for line in text.splitlines()
        if line.strip().startswith(prefix)
    ]
    if len(matches) != 1:
        raise PdfParsingError(message)
    return matches[0]


def parse_financial_pdf(pdf_path):
    path = Path(pdf_path)
    if not path.is_file():
        raise PdfParsingError("PDF не найден")

    size = path.stat().st_size
    if size == 0:
        raise PdfParsingError("PDF пуст")
    if size > MAX_PDF_SIZE:
        raise PdfParsingError("PDF превышает 10 МБ")

    with path.open("rb") as file:
        if file.read(5) != b"%PDF-":
            raise PdfParsingError("Файл не является PDF")

    try:
        with pdfplumber.open(path) as pdf:
            texts = [page.extract_text() or "" for page in pdf.pages]
            text = "\n".join(texts)
            tables = []
            for page in pdf.pages:
                for table in page.extract_tables():
                    if table and [normalized(c) for c in table[0]] == HEADERS:
                        tables.append(table)
    except Exception as exc:
        # Повреждённый, зашифрованный либо некорректно сформированный PDF.
        raise PdfParsingError("Не удалось прочитать PDF") from exc

    if not text.strip():
        raise PdfParsingError(
            "В документе нет извлекаемого текста (возможно, это скан)"
        )

    if len(tables) != 1:
        raise PdfParsingError(
            "Не найдена ровно одна финансовая таблица нужного формата"
        )
    table = tables[0]
    if len(table) != 7:
        raise PdfParsingError("Ожидалось ровно 6 месяцев финансовых данных")

    inn_line = _unique_line(text, "ИНН:", "Ожидается ровно один ИНН")
    inn_match = re.fullmatch(r"ИНН:\s*(\d{10}|\d{12})", inn_line)
    if inn_match is None:
        raise PdfParsingError("ИНН имеет неверный формат")

    debt_line = _unique_line(
        text, DEBT_LABEL, "Ожидается ровно один ежемесячный долговой платёж"
    )
    debt_match = re.fullmatch(
        re.escape(DEBT_LABEL) + r"\s*(" + MONEY_PATTERN + r")\s*руб\.",
        debt_line,
    )
    if debt_match is None:
        raise PdfParsingError("Ежемесячный долговой платёж имеет неверный формат")

    financials = []
    for row in table[1:]:
        if len(row) != 5 or not normalized(row[0]):
            raise PdfParsingError("Недостаточно значений в строке таблицы")
        financials.append(
            MonthlyFinancials(
                period_month=normalized(row[0]),
                revenue_kopecks=money_to_kopecks(row[1]),
                operating_inflow_kopecks=money_to_kopecks(row[2]),
                operating_outflow_kopecks=money_to_kopecks(row[3]),
                tax_outflow_kopecks=money_to_kopecks(row[4]),
            )
        )

    indexes = []
    for item in financials:
        year, month = map(int, item.period_month.split("-"))
        indexes.append(year * 12 + month)
    if any(next_month - previous_month != 1 for previous_month, next_month in zip(indexes, indexes[1:])):
        raise PdfParsingError("Месяцы должны идти подряд, без пропусков")

    return FinancialExtractionResponse(
        inn=inn_match.group(1),
        financials=financials,
        declared_monthly_debt_payment_kopecks=money_to_kopecks(debt_match.group(1)),
    )
