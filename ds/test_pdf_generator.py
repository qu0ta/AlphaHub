
from pathlib import Path

import pdfplumber

from schemas import ScoringInput
from pdf_generator import format_rubles


BASE_DIR = Path(__file__).resolve().parent


def test_pdf(profile):
    json_path = BASE_DIR / "fixtures" / f"{profile}.json"
    pdf_path = BASE_DIR / "demo_pdfs" / f"{profile}.pdf"

    # Загружаем исходные данные из JSON
    data = ScoringInput.model_validate_json(
        json_path.read_text(encoding="utf-8")
    )

    # Проверяем существование и размер PDF
    assert pdf_path.exists(), f"{profile}: PDF не найден"
    assert pdf_path.stat().st_size <= 10 * 1024 * 1024

    # Извлекаем текст
    with pdfplumber.open(pdf_path) as pdf:
        text = "\n".join(
            page.extract_text() or ""
            for page in pdf.pages
        )

    # Проверяем ИНН
    assert data.inn in text

    # Проверяем финансовые показатели за все 6 месяцев
    assert len(data.financials) == 6

    for month in data.financials:
        assert month.period_month in text

        values = [
            month.revenue_kopecks,
            month.operating_inflow_kopecks,
            month.operating_outflow_kopecks,
            month.tax_outflow_kopecks,
        ]

        for value in values:
            assert format_rubles(value) in text

    # Проверяем ежемесячный долговой платёж
    debt = format_rubles(
        data.declared_monthly_debt_payment_kopecks
    )
    assert debt in text

    print(f"{profile}: тест пройден!")


if __name__ == "__main__":
    for profile in ["stable", "medium", "risky"]:
        test_pdf(profile)

    print("Все тесты успешно пройдены!")
