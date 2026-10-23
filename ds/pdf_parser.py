from pathlib import Path
import re
import pdfplumber
from schemas import FinancialExtractionResponse, MonthlyFinancials

HEADERS = ["Месяц", "Выручка", "Поступления", "Операционные выплаты", "Налоги"]
DEBT_LABEL = "Ежемесячный платёж по действующим кредитам:"
MONEY_PATTERN = r"(?:\d{1,3}(?:[ \u00a0\u202f]\d{3})+|\d+),\d{2}"

def normalized(cell):
    return ' '.join(str(cell or '').split())

def money_to_kopecks(value):
    value = normalized(value)
    if re.fullmatch(MONEY_PATTERN, value) is None:
        raise ValueError(f'Некорректная сумма: {value!r}')
    rubles, kopecks = value.split(',')
    return int(rubles.replace(' ', '')) * 100 + int(kopecks)

def parse_financial_pdf(pdf_path):
    path = Path(pdf_path)
    if not path.is_file():
        raise ValueError('PDF не найден')
    if path.stat().st_size > 10 * 1024 * 1024:
        raise ValueError('PDF превышает 10 МБ')
    with pdfplumber.open(path) as pdf:
        texts = [page.extract_text() or '' for page in pdf.pages]
        text = '\n'.join(texts)
        if not text.strip():
            raise ValueError('В документе нет извлекаемого текста')
        tables = []
        for page in pdf.pages:
            for table in page.extract_tables():
                if table and [normalized(c) for c in table[0]] == HEADERS:
                    tables.append(table)

    if len(tables) != 1:
        raise ValueError('Не найдена ровно одна финансовая таблица нужного формата')
    table = tables[0]
    if len(table) != 7:
        raise ValueError('Ожидалось ровно 6 месяцев финансовых данных')
    inn_match = re.search(r'(?m)^ИНН:[ \t]*(\d{10}|\d{12})[ \t]*$', text)
    if inn_match is None:
        raise ValueError('ИНН не найден или имеет неверный формат')
    debt_match = re.search(r'(?m)^' + re.escape(DEBT_LABEL) + r'[ \t]*(' + MONEY_PATTERN + r')[ \t]*руб\.[ \t]*$', text)
    if debt_match is None:
        raise ValueError('Ежемесячный долговой платёж не найден')
    financials=[]
    for row in table[1:]:
        if len(row) != 5 or not normalized(row[0]):
            raise ValueError('Недостаточно значений в строке таблицы')
        financials.append(MonthlyFinancials(
            period_month=normalized(row[0]),
            revenue_kopecks=money_to_kopecks(row[1]),
            operating_inflow_kopecks=money_to_kopecks(row[2]),
            operating_outflow_kopecks=money_to_kopecks(row[3]),
            tax_outflow_kopecks=money_to_kopecks(row[4]),
        ))
    indexes = []
    for item in financials:
        yr, mon = map(int, item.period_month.split('-'))
        indexes.append(yr * 12 + mon)
    if any(b - a != 1 for a,b in zip(indexes, indexes[1:])):
        raise ValueError('Месяцы должны идти подряд, без пропусков')
    return FinancialExtractionResponse(inn=inn_match.group(1), financials=financials, declared_monthly_debt_payment_kopecks=money_to_kopecks(debt_match.group(1)))
