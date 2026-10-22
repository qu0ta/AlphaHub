
from datetime import date
from pathlib import Path

from generator import PROFILES, generate_company_financials


def save_examples():
    # Фиксированная дата для воспроизводимости
    reference_date = date(2026, 10, 22)
    seed = 42

    # Папка для сохранения JSON
    output_dir = Path(__file__).resolve().parent / "fixtures"
    output_dir.mkdir(exist_ok=True)

    for profile in PROFILES:
        company = generate_company_financials(
            profile=profile,
            seed=seed,
            reference_date=reference_date
        )

        file_path = output_dir / f"{profile}.json"

        file_path.write_text(
            company.model_dump_json(indent=2),
            encoding="utf-8"
        )

        print(f"Создан файл: {file_path.name}")


if __name__ == "__main__":
    save_examples()
