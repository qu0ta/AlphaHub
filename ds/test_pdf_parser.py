from pathlib import Path

from schemas import ScoringInput
from pdf_parser import parse_financial_pdf, money_to_kopecks

BASE_DIR = Path(__file__).resolve().parent


def test_profile(profile: str) -> None:
    json_path = BASE_DIR / "fixtures" / f"{profile}.json"
    pdf_path = BASE_DIR / "demo_pdfs" / f"{profile}.pdf"

    expected = ScoringInput.model_validate_json(
        json_path.read_text(encoding="utf-8")
    )
    actual = parse_financial_pdf(pdf_path)

    assert actual.inn == expected.inn
    assert [x.model_dump() for x in actual.financials] == [
        x.model_dump() for x in expected.financials
    ]
    assert (
        actual.declared_monthly_debt_payment_kopecks
        == expected.declared_monthly_debt_payment_kopecks
    )

    print(f"{profile}: данные PDF полностью совпадают с JSON")


if __name__ == "__main__":
    assert money_to_kopecks("913 348,66") == 91334866
    assert money_to_kopecks("0,01") == 1

    for profile in ("stable", "medium", "risky"):
        test_profile(profile)

    print("Все тесты PDF-парсера пройдены!")
