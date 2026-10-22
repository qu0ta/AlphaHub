
from datetime import date
from pydantic import BaseModel, Field, field_validator, model_validator


def get_last_six_months(reference_date: date) -> list[str]:
    year = reference_date.year
    month = reference_date.month
    months = []

    for _ in range(6):
        month -= 1

        if month == 0:
            month = 12
            year -= 1

        months.append(f"{year:04d}-{month:02d}")

    months.reverse()
    return months


class MonthlyFinancials(BaseModel):
    period_month: str
    revenue_kopecks: int = Field(ge=0)
    operating_inflow_kopecks: int = Field(ge=0)
    operating_outflow_kopecks: int = Field(ge=0)
    tax_outflow_kopecks: int = Field(ge=0)

    @field_validator("period_month")
    @classmethod
    def validate_period_month(cls, value: str) -> str:
        try:
            year, month = map(int, value.split("-"))
            date(year, month, 1)

            if value != f"{year:04d}-{month:02d}":
                raise ValueError()

        except (ValueError, TypeError):
            raise ValueError("Месяц должен быть в формате YYYY-MM")

        return value


class ScoringInput(BaseModel):
    # Из данных компании 
    inn: str

    # Из Application на Go
    amount_kopecks: int = Field(gt=0)
    term_months: int = Field(gt=0)

    # Ставка оцениваемого кредита: 1990 = 19.90% годовых.
    annual_rate_bps: int = Field(ge=0, strict=True)

    # Дата первоначальной подачи, передаётся бэком как YYYY-MM-DD.
    # При повторном расчёте сохраняется; не подменяется текущей датой.
    application_date: date = Field(ge=date(1, 7, 1))

    # Финансовые данные
    financials: list[MonthlyFinancials] = Field(
        min_length=6,
        max_length=6
    )

    declared_monthly_debt_payment_kopecks: int = Field(ge=0)

    @model_validator(mode="after")
    def validate_financials(self):
        expected = get_last_six_months(self.application_date)

        actual = [
            record.period_month
            for record in self.financials
        ]

        if actual != expected:
            raise ValueError(
                "Ожидаются 6 полных месяцев перед датой подачи, по порядку: "
                + ", ".join(expected)
            )

        return self


class FinancialExtractionResponse(BaseModel):
    inn: str
    financials: list[MonthlyFinancials] = Field(
        min_length=6,
        max_length=6
    )
    declared_monthly_debt_payment_kopecks: int = Field(ge=0)


class ScoringExplanation(BaseModel):
    positive_factors: list[str]
    negative_factors: list[str]


class ScoringResponse(BaseModel):
    score: int = Field(ge=0, le=100)
    explanation: ScoringExplanation
    model_version: str
