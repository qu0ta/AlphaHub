"""Исследование структуры текстовых PDF при помощи Docling.

Не изменяет pdf_parser.py и не создаёт FinancialExtractionResponse:
Docling выделяет структуру/таблицы, но не восстанавливает отсутствующие
помесячные финансовые показатели.

Примеры из папки ds (с активной .venv-docling):
    python docling_probe.py real_pdf_samples/arenadata_6m2025.pdf --pages 5 5
    python docling_probe.py demo_pdfs/stable.pdf --pages 1 1
"""

import argparse
from pathlib import Path

from docling.datamodel.base_models import InputFormat
from docling.datamodel.pipeline_options import PdfPipelineOptions
from docling.document_converter import DocumentConverter, PdfFormatOption


MAX_FILE_SIZE = 10 * 1024 * 1024
KEYWORDS = ("выруч", "поступлен", "операцион", "налог", "долгов", "кредит")


def main():
    parser = argparse.ArgumentParser(
        description="Посмотреть, как Docling распознаёт финансовые таблицы в PDF"
    )
    parser.add_argument("pdf", type=Path, help="Путь к PDF-файлу")
    parser.add_argument(
        "--pages", nargs=2, metavar=("FIRST", "LAST"), type=int,
        default=[1, 1], help="Номера страниц включительно, начиная с 1 (по умолчанию 1 1)"
    )
    parser.add_argument(
        "--output", type=Path, default=Path("docling_output"),
        help="Каталог для Markdown и CSV с найденными таблицами"
    )
    args = parser.parse_args()

    pdf_path = args.pdf.expanduser().resolve()
    if not pdf_path.is_file():
        parser.error(f"Файл не найден: {pdf_path}")
    if pdf_path.suffix.lower() != ".pdf":
        parser.error("Требуется файл с расширением .pdf")
    if pdf_path.stat().st_size > MAX_FILE_SIZE:
        parser.error("Файл превышает 10 МБ")

    first, last = args.pages
    if first < 1 or last < first:
        parser.error("Диапазон страниц должен быть положительным: --pages 5 5")

    # Работаем с текстовыми PDF. OCR в этом эксперименте выключен.
    pipeline = PdfPipelineOptions()
    pipeline.do_ocr = False
    pipeline.do_table_structure = True

    converter = DocumentConverter(
        format_options={
            InputFormat.PDF: PdfFormatOption(pipeline_options=pipeline),
        }
    )

    print(f"Файл: {pdf_path.name}")
    print(f"Обрабатываем страницы: {first}–{last}")
    result = converter.convert(
        pdf_path,
        page_range=(first, last),
        max_num_pages=100,
        max_file_size=MAX_FILE_SIZE,
    )
    doc = result.document

    args.output.mkdir(parents=True, exist_ok=True)
    prefix = f"{pdf_path.stem}_p{first}-{last}"

    markdown = doc.export_to_markdown()
    markdown_path = args.output / f"{prefix}.md"
    markdown_path.write_text(markdown, encoding="utf-8")

    print(f"\nТаблиц распознано: {len(doc.tables)}")
    print(f"Структурированный текст: {markdown_path}")

    for index, table in enumerate(doc.tables, start=1):
        csv_path = args.output / f"{prefix}_table_{index}.csv"
        dataframe = table.export_to_dataframe(doc=doc)
        dataframe.to_csv(csv_path, index=False, encoding="utf-8-sig")
        print(f"Таблица {index}: {csv_path} ({len(dataframe)} строк)")

    print("\nСтроки, похожие на финансовые показатели:")
    matches = [
        line.strip()
        for line in markdown.splitlines()
        if any(word in line.lower() for word in KEYWORDS)
    ]
    for line in matches[:12]:
        print("  ", line[:220])
    if not matches:
        print("  Не найдены на выбранных страницах")

    print("\nПроверяй структуру таблиц и единицы измерения в созданных файлах.")
    print("Docling сам по себе НЕ создаёт шесть помесячных записей для скоринга.")


if __name__ == "__main__":
    main()
