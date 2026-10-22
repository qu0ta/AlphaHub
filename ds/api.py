import logging

from fastapi import FastAPI, HTTPException

from schemas import ScoringInput, DetailedScoringResponse
from scoring import calculate_score


logger = logging.getLogger(__name__)

app = FastAPI(
    title="AlphaHub Scoring API",
    version="1.0.0",
    description=(
        "Финансовый скоринг. "
        "Результат не является окончательным решением по кредиту."
    ),
)


@app.post(
    "/scoring/evaluate",
    response_model=DetailedScoringResponse,
    tags=["Scoring"],
    summary="Рассчитать финансовый скоринг",
    responses={
        422: {"description": "Некорректные данные или невозможен расчёт"},
        500: {"description": "Внутренняя ошибка сервиса"},
    },
)
def evaluate_scoring(data: ScoringInput) -> DetailedScoringResponse:
    try:
        return calculate_score(data)

    except ValueError as exc:
        # В текущем расчёте ValueError означает:
        # невозможно определить покрытие долговых платежей.
        raise HTTPException(
            status_code=422,
            detail={
                "code": "SCORING_NOT_CALCULABLE",
                "message": "По переданным данным невозможно рассчитать скоринг",
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
    