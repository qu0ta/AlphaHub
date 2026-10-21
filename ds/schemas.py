
from datetime import date
from pydantic import BaseModel, Field, field_validator, model_validator


class MonthlyFinancials(BaseModel):
    period_month: str
    revenue_kopecks: int = Field(ge=0)
    operating_inflow_kopecks: int = Field(ge=0)
    operating_outflow_kopecks: int = Field(ge=0)
    tax_outflow_kopecks: int = Field(ge=0)

    @field_validator("period_month")
    @classmethod
    def validate_period_month(cls, value):
        if len(value) != 7 or value[4] != "-":
            raise ValueError("Месяц должен быть в формате YYYY-MM")

        try:
            year = int(value[:4])
            month = int(value[5:])

            if value != f"{year:04d}-{month:02d}":
                raise ValueError()

            date(year, month, 1)

        except ValueError:
            raise ValueError("Некорректный месяц")

        return value


class ScoringInput(BaseModel):
    inn: str

    financials: list[MonthlyFinancials] = Field(
        min_length=6,
        max_length=6
    )

    declared_monthly_debt_payment_kopecks: int = Field(ge=0)

    requested_amount_kopecks: int = Field(gt=0)
    requested_term_months: int = Field(gt=0)

    @model_validator(mode="after")
    def validate_financials(self):
        today = date.today()

        year = today.year
        month = today.month

        expected_months = []

        for i in range(6):
            month -= 1

            if month == 0:
                month = 12
                year -= 1

            expected_months.append(f"{year:04d}-{month:02d}")

        expected_months.reverse()

        actual_months = [
            record.period_month
            for record in self.financials
        ]

        if actual_months != expected_months:
            raise ValueError(
                "Требуются последние 6 полных месяцев "
                "в хронологическом порядке: "
                + ", ".join(expected_months)
            )

        return self
