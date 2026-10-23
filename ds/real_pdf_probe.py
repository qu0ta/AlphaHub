"""Пробный разбор настоящего PDF в DS-07/DS-08.

Запуск из папки ds:
    ../.venv/bin/python real_pdf_probe.py real_pdf_samples/arenadata_6m2025.pdf
"""

from pathlib import Path
import sys

import pdfplumber

from pdf_parser import PdfParsingError, parse_financial_pdf


def main() -> None:
    if len(sys.argv) != 2:
        print("Использование: python real_pdf_probe.py <путь_к_pdf>")
        sys.exit(2)

    path = Path(sys.argv[1])
    if not path.is_file():
        print(f"Файл не найден: {path}")
        sys.exit(2)

    print(f"Документ: {path.name}")
    try:
        with pdfplumber.open(path) as pdf:
            print(f"Страниц: {len(pdf.pages)}")
            found_text = False
            found_revenue = False

            for page_number, page in enumerate(pdf.pages, start=1):
                text = page.extract_text() or ""
                if text.strip():
                    found_text = True
                revenue_lines = [
                    line.strip()
                    for line in text.splitlines()
                    if "Выручка" in line or "выручка" in line
                ]
                if revenue_lines:
                    found_revenue = True
                    print(f"\nСтраница {page_number}: строки с выручкой:")
                    for line in revenue_lines[:5]:
                        print("  ", line)
                    if found_revenue:
                        break

            if not found_text:
                print("Не удалось извлечь текстовый слой (возможно, скан).")
            elif not found_revenue:
                print("Текст извлекается, но строки со словом 'выручка' не найдены.")
    except Exception as exc:
        print(f"Не удалось прочитать файл библиотекой pdfplumber: {exc}")
        sys.exit(1)

    print("\nПроверка соответствия нашему шаблону:")
    try:
        result = parse_financial_pdf(path)
    except PdfParsingError as exc:
        print(f"Документ не подходит для автоматического извлечения: {exc}")
        print("Это корректный результат, если нет шести помесячных строк нужного формата.")
    else:
        print("Документ подходит! Извлечённые данные:")
        print(result.model_dump_json(indent=2))


if __name__ == "__main__":
    main()
