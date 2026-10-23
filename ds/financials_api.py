"""HTTP-маршрут извлечения финансов из текстового PDF.

Подключается к существующему FastAPI из api.py через app.include_router(router).
Не импортирует Docling при запуске API: Docling нужен только при fallback.
"""

from pathlib import Path
from tempfile import TemporaryDirectory

from fastapi import APIRouter, File, HTTPException, UploadFile
from pydantic import ValidationError

from flexible_pdf_parser import FlexiblePdfError, parse_flexible_pdf
from pdf_parser import PdfParsingError, parse_financial_pdf
from schemas import FinancialExtractionResponse

router = APIRouter()
MAX_PDF_SIZE = 10 * 1024 * 1024
UNSUPPORTED_TEMPLATE = "Не найдена ровно одна финансовая таблица нужного формата"


def _reject(status_code: int, code: str, message: str) -> None:
    """Формат ошибок совпадает с обработчиком HTTPException в api.py."""
    raise HTTPException(
        status_code=status_code,
        detail={"code": code, "message": message},
    )


@router.post(
    "/api/v1/financials/extract",
    response_model=FinancialExtractionResponse,
    tags=["Financial extraction"],
    summary="Извлечь финансовые показатели из PDF",
)
def extract_financials(file: UploadFile = File(...)) -> FinancialExtractionResponse:
    """Принимает multipart/form-data с полем file (текстовый PDF <= 10 МБ).

    Сначала пробует быстрый парсер известной таблицы, затем — Docling
    для альтернативной структуры. Неполные или некорректные данные
    никогда не дополняются догадками.
    """
    try:
        # Читаем не более ограничения + 1 байт: большой файл не попадёт целиком в RAM.
        content = file.file.read(MAX_PDF_SIZE + 1)
    finally:
        file.file.close()

    if not content:
        _reject(400, "EMPTY_FILE", "Загруженный PDF пуст")
    if len(content) > MAX_PDF_SIZE:
        _reject(413, "FILE_TOO_LARGE", "Размер PDF превышает 10 МБ")
    if file.content_type not in (None, "application/pdf", "application/octet-stream"):
        _reject(415, "UNSUPPORTED_MEDIA_TYPE", "Требуется PDF-документ")
    if not content.startswith(b"%PDF-"):
        _reject(415, "UNSUPPORTED_MEDIA_TYPE", "Файл не является PDF")

    # Используем только созданный нами путь: имя загруженного файла недоверенное.
    with TemporaryDirectory(prefix="alphahub_pdf_") as temp_dir:
        path = Path(temp_dir) / "document.pdf"
        path.write_bytes(content)

        try:
            return parse_financial_pdf(path)
        except PdfParsingError as exc:
            # Fallback только при неподдерживаемом расположении таблицы.
            # Ошибки данных стандартного формата НЕ маскируем второй попыткой.
            if str(exc) != UNSUPPORTED_TEMPLATE:
                _reject(422, "PDF_EXTRACTION_ERROR", str(exc))
        except ValidationError:
            _reject(422, "PDF_EXTRACTION_ERROR", "Некорректные финансовые данные")

        try:
            return parse_flexible_pdf(path)
        except FlexiblePdfError as exc:
            if str(exc).startswith("Не установлен Docling"):
                _reject(503, "EXTRACTION_SERVICE_UNAVAILABLE", "Docling не установлен в DS-сервисе")
            _reject(422, "PDF_EXTRACTION_ERROR", str(exc))
        except ValidationError:
            _reject(422, "PDF_EXTRACTION_ERROR", "Некорректные финансовые данные")
