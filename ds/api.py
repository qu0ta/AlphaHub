import logging

from fastapi import FastAPI, HTTPException, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from starlette.exceptions import HTTPException as StarletteHTTPException

from schemas import ScoringInput, ScoringResponse
from scoring import score_application


logger = logging.getLogger(__name__)

app = FastAPI(
    title="AlphaHub Scoring API",
    version="1.0.0",
    description=(
        "Финансовый скоринг. "
        "Результат не является окончательным решением по кредиту."
    ),
)


def error_response(
    status_code: int,
    code: str,
    message: str,
    headers: dict[str, str] | None = None,
) -> JSONResponse:
    return JSONResponse(
        status_code=status_code,
        content={
            "error": {
                "code": code,
                "message": message,
            }
        },
        headers=headers,
    )


@app.exception_handler(RequestValidationError)
async def validation_error_handler(
    request: Request,
    exc: RequestValidationError,
) -> JSONResponse:
    messages = []

    for error in exc.errors():
        location = ".".join(str(part) for part in error["loc"])
        messages.append(f"{location}: {error['msg']}")

    return error_response(
        status_code=422,
        code="VALIDATION_ERROR",
        message="; ".join(messages) or "Некорректные входные данные",
    )


@app.exception_handler(StarletteHTTPException)
async def http_error_handler(
    request: Request,
    exc: StarletteHTTPException,
) -> JSONResponse:
    if isinstance(exc.detail, dict):
        code = str(exc.detail.get("code") or "HTTP_ERROR")
        message = str(exc.detail.get("message") or "Ошибка запроса")
    else:
        code = "HTTP_ERROR"
        message = str(exc.detail) or "Ошибка запроса"

    return error_response(
        status_code=exc.status_code,
        code=code,
        message=message,
        headers=exc.headers,
    )


@app.exception_handler(Exception)
async def unexpected_error_handler(
    request: Request,
    exc: Exception,
) -> JSONResponse:
    logger.error(
        "Unhandled API error",
        exc_info=(type(exc), exc, exc.__traceback__),
    )

    return error_response(
        status_code=500,
        code="INTERNAL_ERROR",
        message="Внутренняя ошибка сервиса",
    )


@app.post(
    "/api/v1/scoring/evaluate",
    response_model=ScoringResponse,
    tags=["Scoring"],
)
def evaluate_scoring(data: ScoringInput) -> ScoringResponse:
    try:
        return score_application(data)

    except ValueError as exc:
        raise HTTPException(
            status_code=422,
            detail={
                "code": "SCORING_NOT_CALCULABLE",
                "message": (
                    "По переданным данным невозможно рассчитать скоринг"
                ),
            },
        ) from exc

    except Exception as exc:
        logger.exception("Unexpected scoring error")

        raise HTTPException(
            status_code=500,
            detail={
                "code": "INTERNAL_ERROR",
                "message": "Внутренняя ошибка расчёта скоринга",
            },
        ) from exc

#для парсера
from financials_api import router as financials_router

app.include_router(financials_router)