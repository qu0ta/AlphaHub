
from datetime import date
from pathlib import Path

from generator import generate_company_financials
from schemas import ScoringInput, get_last_six_months


REFERENCE_DATE = date(2026, 10, 22)
SEED = 42


def generate(profile):
    return generate_company_financials(
        profile=profile,
        seed=SEED,
        reference_date=REFERENCE_DATE
    )


def cashflow(month):
    return (
        month.operating_inflow_kopecks
        - month.operating_outflow_kopecks
        - month.tax_outflow_kopecks
    )


stable = generate("stable")
medium = generate("medium")
risky = generate("risky")

# 1. Проверяем шесть последовательных месяцев
for company in [stable, medium, risky]:
    assert len(company.financials) == 6

    actual = [
        month.period_month
        for month in company.financials
    ]

    assert actual == get_last_six_months(REFERENCE_DATE)

print("Тест 1: шесть месяцев — OK")


# 2. Проверяем воспроизводимость
assert stable == generate("stable")
assert medium == generate("medium")
assert risky == generate("risky")

print("Тест 2: воспроизводимость — OK")


# 3. Stable: положительный денежный поток
for month in stable.financials:
    assert cashflow(month) > 0

print("Тест 3: stable — OK")


# 4. Medium: сильные колебания выручки
stable_revenues = [
    m.revenue_kopecks for m in stable.financials
]

medium_revenues = [
    m.revenue_kopecks for m in medium.financials
]

stable_range = max(stable_revenues) - min(stable_revenues)
medium_range = max(medium_revenues) - min(medium_revenues)

assert medium_range > stable_range

print("Тест 4: medium — OK")


# 5. Risky: падение выручки и отрицательный поток
risky_revenues = [
    m.revenue_kopecks for m in risky.financials
]

for i in range(1, 6):
    assert risky_revenues[i] < risky_revenues[i - 1]

for month in risky.financials:
    assert cashflow(month) < 0

print("Тест 5: risky — OK")


# 6. Проверяем возможность менять параметры
custom = generate_company_financials(
    profile="risky",
    seed=SEED,
    reference_date=REFERENCE_DATE,
    overrides={"monthly_trend": -0.18}
)

assert custom != risky

print("Тест 6: настройка параметров — OK")


# 7. Проверяем сохранённые JSON
fixtures_dir = Path(__file__).resolve().parent / "fixtures"

for profile in ["stable", "medium", "risky"]:
    path = fixtures_dir / f"{profile}.json"

    saved = ScoringInput.model_validate_json(
        path.read_text(encoding="utf-8")
    )

    assert saved == generate(profile)

print("Тест 7: эталонные JSON — OK")

print("\nВсе тесты DS-03 пройдены!")
