import json
import math
from dataclasses import asdict, dataclass, field
from pathlib import Path


@dataclass(frozen=True)
class Scenario:
    observation_noise: float = 0.08
    volatility_multiplier: float = 1.0
    seasonal_amplitude: float = 0.15
    shock_probability: float = 0.10
    shock_scale: float = 0.25
    inflow_multiplier: float = 1.0
    expense_multiplier: float = 1.0
    risk_shift: float = 0.0
    hidden_risk_std: float = 0.65

    def __post_init__(self):
        if not all(math.isfinite(value) for value in asdict(self).values()):
            raise ValueError("Параметры сценария должны быть конечными числами")
        if not 0 <= self.shock_probability <= 1 or not 0 <= self.seasonal_amplitude < 1:
            raise ValueError("Недопустимая вероятность шока или амплитуда сезонности")
        if min(self.observation_noise, self.shock_scale, self.hidden_risk_std) < 0:
            raise ValueError("Шум и величина шока не могут быть отрицательными")
        if min(self.volatility_multiplier, self.inflow_multiplier, self.expense_multiplier) <= 0:
            raise ValueError("Множители сценария должны быть положительными")


def default_scenarios():
    return {
        "base": Scenario(),
        "high_noise": Scenario(observation_noise=0.28, volatility_multiplier=1.8,
                               seasonal_amplitude=0.30, shock_probability=0.20, shock_scale=0.50),
        "stress": Scenario(observation_noise=0.16, volatility_multiplier=1.4,
                           shock_probability=0.20, shock_scale=0.45,
                           inflow_multiplier=0.82, expense_multiplier=1.22, risk_shift=0.65),
    }


@dataclass(frozen=True)
class Economics:
    net_yield: float = 0.08
    lgd: float = 0.45
    manual_review_cost_kopecks: int = 100_000

    def __post_init__(self):
        if any(not math.isfinite(value) or not 0 <= value <= 1 for value in (self.net_yield, self.lgd)):
            raise ValueError("Чистая доходность и LGD должны быть от 0 до 1")
        if self.manual_review_cost_kopecks < 0:
            raise ValueError("Стоимость ручной проверки не может быть отрицательной")


@dataclass(frozen=True)
class ExperimentConfig:
    samples: int = 6000
    seeds: tuple[int, ...] = (42, 43, 44)
    horizon_months: int = 12
    economics: Economics = field(default_factory=Economics)
    scenarios: dict[str, Scenario] = field(default_factory=default_scenarios)

    def __post_init__(self):
        if self.samples < 5000:
            raise ValueError("Полное исследование требует не менее 5000 заявок на выборку")
        if len(set(self.seeds)) < 3 or len(set(self.seeds)) != len(self.seeds):
            raise ValueError("Нужно минимум три разных seed")
        if any(type(seed) is not int or not 0 <= seed < 2**31 for seed in self.seeds):
            raise ValueError("seed должен быть целым числом от 0 до 2**31 - 1")
        if self.horizon_months != 12:
            raise ValueError("В этом исследовании зафиксирован 12-месячный горизонт дефолта")
        if set(self.scenarios) != {"base", "high_noise", "stress"}:
            raise ValueError("Нужны сценарии base, high_noise, stress")

    def to_dict(self):
        return asdict(self)

    @classmethod
    def from_file(cls, path: Path):
        payload = json.loads(path.read_text(encoding="utf-8"))
        if "economics" in payload:
            payload["economics"] = Economics(**payload["economics"])
        if "scenarios" in payload:
            scenarios = default_scenarios()
            for name, values in payload["scenarios"].items():
                if name not in scenarios:
                    raise ValueError(f"Неизвестный сценарий: {name}")
                scenarios[name] = Scenario(**{**asdict(scenarios[name]), **values})
            payload["scenarios"] = scenarios
        if "seeds" in payload:
            payload["seeds"] = tuple(payload["seeds"])
        return cls(**payload)
