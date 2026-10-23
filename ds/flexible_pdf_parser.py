"""Дополнительное извлечение финансов из таблиц Docling.

Поддерживаются ПОМЕСЯЧНЫЕ финансовые сводки с переставленными колонками,
несколькими вариантами заголовков, а также таблицы с месяцами в столбцах.
Не заполняет отсутствующие данные.

Docling импортируется только при вызове parse_flexible_pdf, поэтому
обработка экспортированных CSV/Markdown работает без установленного Docling.
"""

import csv
import re
from decimal import Decimal, InvalidOperation
from pathlib import Path
from typing import Iterable, Mapping

from pydantic import ValidationError
from schemas import FinancialExtractionResponse, MonthlyFinancials

MAX_PDF_BYTES = 10 * 1024 * 1024


class FlexiblePdfError(ValueError):
    """Финансовые данные нельзя восстановить однозначно."""


# Только эквивалентные по смыслу названия. Себестоимость и бухгалтерская
# прибыль НЕ заменяют операционные денежные выплаты/поступления.
ALIASES = {
    "period_month": (
        "месяц", "период", "отчетный месяц", "отчётный месяц"
    ),
    "revenue_kopecks": (
        "выручка", "выручка от продаж", "выручка от реализации",
    ),
    "operating_inflow_kopecks": (
        "поступления", "операционные поступления",
        "поступления от основной деятельности",
        "поступления от операционной деятельности",
        "денежные поступления от основной деятельности",
    ),
    "operating_outflow_kopecks": (
        "операционные выплаты", "операционные платежи",
        "операционные выплаты без налогов и кредитных платежей",
        "операционные выплаты без налогов и кредитов",
        "операционные платежи без налогов и кредитных платежей",
    ),
    "tax_outflow_kopecks": (
        "налоги", "налоговые выплаты", "уплаченные налоги",
        "налоговые платежи",
    ),
}


MONTH_RE = re.compile(r"\d{4}-(?:0[1-9]|1[0-2])\Z")
METRIC_COLUMN_ALIASES = {"показатель", "финансовый показатель", "наименование показателя", "статья"}

AMOUNT_RE = re.compile(
    r"(?:\d{1,3}(?:[ \u00a0\u202f]\d{3})+|\d+)(?:[,.]\d{2})?\Z"
)


def _norm(text: object) -> str:
    return " ".join(str(text or "").replace("\ufeff", "").lower().replace("ё", "е").split()).strip(" :")


def _headers(columns: Iterable[str]) -> dict:
    result = {}
    for column in columns:
        label = _norm(column)
        matches = [
            field for field, labels in ALIASES.items()
            if label in {_norm(name) for name in labels}
        ]
        if len(matches) == 1:
            field = matches[0]
            if field in result:
                raise FlexiblePdfError("В таблице повторяется финансовый показатель: " + field)
            result[field] = column

    if len(result) != len(ALIASES):
        absent = [name for name in ALIASES if name not in result]
        raise FlexiblePdfError("Отсутствуют обязательные столбцы: " + ", ".join(absent))
    return result


def _unit_scale(markdown: str) -> int:
    """Копеек в одной единице таблицы: 100 или 100_000."""
    # Подпись к таблице должна однозначно говорить, что суммы указаны
    # в рублях или тысячах рублей. Единицу долга проверяем отдельно.
    lines = [
        _norm(line) for line in markdown.splitlines()
        if "плат" not in _norm(line) or "кредит" not in _norm(line)
    ]
    thousand = any(
        re.search(r"(?:тыс\.?|тысяч\w*)\s*(?:российских\s*)?руб", line)
        for line in lines
    )
    rubles = any(
        "суммы в рублях" in line or "сумма в рублях" in line
        or "показатели в рублях" in line
        for line in lines
    )
    if thousand and rubles:
        raise FlexiblePdfError("Неоднозначно указаны единицы измерения")
    if thousand:
        return 100_000
    if rubles:
        return 100
    raise FlexiblePdfError("Не указаны единицы финансовой таблицы (рубли или тыс. рублей)")


def _money(value: object, scale: int) -> int:
    if value is None:
        raise FlexiblePdfError("Пропущено денежное значение")
    string = " ".join(str(value).replace("\u202f", " ").replace("\u00a0", " ").split())
    if not AMOUNT_RE.fullmatch(string):
        raise FlexiblePdfError(f"Некорректное денежное значение: {string!r}")
    try:
        amount = Decimal(string.replace(" ", "").replace(",", ".")) * scale
    except InvalidOperation as exc:
        raise FlexiblePdfError(f"Некорректная сумма: {string!r}") from exc
    if amount < 0 or amount != amount.to_integral_value():
        raise FlexiblePdfError(f"Некорректная точность или знак суммы: {string!r}")
    return int(amount)


def _metadata(markdown: str):
    inn_matches = re.findall(r"(?m)^\s*ИНН\s*:\s*(\d{10}|\d{12})\s*$", markdown, flags=re.I)
    if len(inn_matches) != 1:
        raise FlexiblePdfError("Ожидается ровно один ИНН из 10 или 12 цифр")
    debt_matches = re.findall(
        r"(?mi)^\s*Ежемесячн(?:ый|ые)\s+плат[её]ж(?:и)?\s+по\s+(?:действующим\s+)?кредитам\s*:\s*(.*?)\s*$",
        markdown,
    )
    if len(debt_matches) != 1:
        raise FlexiblePdfError("Ожидается ровно один ежемесячный платёж по кредитам")

    debt_raw = debt_matches[0].strip()
    match = re.fullmatch(r"(.+?)\s*(тыс\.?\s*руб\.?|руб\.?|₽)", debt_raw, flags=re.I)
    if match is None:
        raise FlexiblePdfError("У долгового платежа отсутствуют единицы измерения")
    scale = 100_000 if re.search(r"тыс", match.group(2), re.I) else 100
    return inn_matches[0], _money(match.group(1), scale)


def _validate_month_sequence(financials: list[MonthlyFinancials]) -> None:
    month_numbers = [int(x.period_month[:4]) * 12 + int(x.period_month[5:]) for x in financials]
    if any(b - a != 1 for a, b in zip(month_numbers, month_numbers[1:])):
        raise FlexiblePdfError("Месяцы должны идти подряд по возрастанию")


def _parse_standard_table(rows: list[Mapping[str, str]], headers: dict, scale: int) -> list[MonthlyFinancials]:
    """Традиционная ориентация: шесть строк-месяцев и пять столбцов."""
    if len(rows) != 6:
        raise FlexiblePdfError("Финансовая таблица должна содержать ровно шесть месяцев")

    financials = []
    for row in rows:
        try:
            record = {"period_month": str(row[headers["period_month"]]).strip()}
            for field in ALIASES:
                if field != "period_month":
                    record[field] = _money(row[headers[field]], scale)
            financials.append(MonthlyFinancials(**record))
        except (KeyError, ValidationError, TypeError, ValueError) as exc:
            if isinstance(exc, FlexiblePdfError):
                raise
            raise FlexiblePdfError("Ошибка в месяце или финансовых показателях") from exc

    _validate_month_sequence(financials)
    return financials


def _transposed_columns(columns: Iterable[str]) -> tuple[str, list[str]] | None:
    """Ищет заголовок 'Показатель' и шесть столбцов YYYY-MM."""
    columns = list(columns)
    metric_keys = [name for name in columns if _norm(name) in METRIC_COLUMN_ALIASES]
    if not metric_keys:
        return None

    months = [name for name in columns if MONTH_RE.fullmatch(str(name).strip())]
    if not months:
        return None
    if len(metric_keys) != 1:
        raise FlexiblePdfError("Неоднозначный столбец названий показателей")
    if len(months) != 6:
        raise FlexiblePdfError("Транспонированная таблица должна содержать ровно шесть месяцев")
    if len({str(month).strip() for month in months}) != 6:
        raise FlexiblePdfError("Повторяющиеся месяцы в заголовках")
    return metric_keys[0], months


def _parse_transposed_table(rows: list[Mapping[str, str]], metric_column: str,
                            months: list[str], scale: int) -> list[MonthlyFinancials]:
    """Транспонированная ориентация: 4 строки-показателя и 6 столбцов-месяцев."""
    metric_fields = {field for field in ALIASES if field != "period_month"}
    metric_rows = {}
    for row in rows:
        title = _norm(row.get(metric_column))
        matches = [
            field for field in metric_fields
            if title in {_norm(alias) for alias in ALIASES[field]}
        ]
        if not matches:
            continue  # Дополнительные строки (например, пояснения) не используем.
        field = matches[0]
        if field in metric_rows:
            raise FlexiblePdfError("Повторяется финансовый показатель: " + field)
        metric_rows[field] = row

    missing = metric_fields - metric_rows.keys()
    if missing:
        raise FlexiblePdfError("Отсутствуют финансовые показатели: " + ", ".join(sorted(missing)))

    financials = []
    for month in months:
        record = {"period_month": str(month).strip()}
        for field in metric_fields:
            record[field] = _money(metric_rows[field].get(month), scale)
        try:
            financials.append(MonthlyFinancials(**record))
        except (ValidationError, TypeError, ValueError) as exc:
            raise FlexiblePdfError("Ошибка в месяце или финансовых показателях") from exc

    _validate_month_sequence(financials)
    return financials


def parse_docling_data(markdown: str, tables: Iterable[Iterable[Mapping[str, str]]]) -> FinancialExtractionResponse:
    """Конвертирует текст и таблицы Docling в согласованную схему.

    tables: набор таблиц; каждая — список словарей `заголовок -> значение`.
    Поддерживает месяцы как в строках, так и в заголовках столбцов.
    """
    inn, debt = _metadata(markdown)
    scale = _unit_scale(markdown)
    found = []
    for rows_iter in tables:
        rows = list(rows_iter)
        if not rows:
            continue
        try:
            headers = _headers(rows[0].keys())
        except FlexiblePdfError as exc:
            if not str(exc).startswith("Отсутствуют обязательные столбцы"):
                raise
        else:
            found.append(_parse_standard_table(rows, headers, scale))
            continue

        transposed = _transposed_columns(rows[0].keys())
        if transposed is not None:
            metric_column, months = transposed
            found.append(_parse_transposed_table(rows, metric_column, months, scale))

    if len(found) != 1:
        raise FlexiblePdfError("Ожидается ровно одна помесячная финансовая таблица поддерживаемого типа")

    return FinancialExtractionResponse(
        inn=inn,
        financials=found[0],
        declared_monthly_debt_payment_kopecks=debt,
    )


def parse_docling_exports(markdown_path: str | Path, csv_paths: Iterable[str | Path]) -> FinancialExtractionResponse:
    """Тестируем сохранённый экспорт Docling без повторного запуска моделей."""
    markdown = Path(markdown_path).read_text(encoding="utf-8")
    tables = []
    for path in csv_paths:
        with Path(path).open(encoding="utf-8-sig", newline="") as file:
            tables.append(list(csv.DictReader(file)))
    return parse_docling_data(markdown, tables)


def parse_flexible_pdf(pdf_path: str | Path) -> FinancialExtractionResponse:
    """Вызывает Docling и возвращает валидированные данные (локально)."""
    path = Path(pdf_path)
    if not path.is_file() or path.stat().st_size == 0 or path.stat().st_size > MAX_PDF_BYTES:
        raise FlexiblePdfError("PDF отсутствует, пуст или превышает 10 МБ")
    with path.open("rb") as stream:
        if stream.read(5) != b"%PDF-":
            raise FlexiblePdfError("Файл не является PDF")

    try:
        from docling.datamodel.base_models import InputFormat
        from docling.datamodel.pipeline_options import PdfPipelineOptions
        from docling.document_converter import DocumentConverter, PdfFormatOption
    except ImportError as exc:
        raise FlexiblePdfError("Не установлен Docling — запусти из .venv-docling") from exc

    pipeline = PdfPipelineOptions()
    pipeline.do_ocr = False
    pipeline.do_table_structure = True
    converter = DocumentConverter(format_options={
        InputFormat.PDF: PdfFormatOption(pipeline_options=pipeline),
    })
    try:
        result = converter.convert(path, max_file_size=MAX_PDF_BYTES, max_num_pages=20)
        doc = result.document
        tables = [t.export_to_dataframe(doc=doc).to_dict(orient="records") for t in doc.tables]
        return parse_docling_data(doc.export_to_markdown(), tables)
    except FlexiblePdfError:
        raise
    except Exception as exc:
        raise FlexiblePdfError("Docling не смог обработать документ") from exc
