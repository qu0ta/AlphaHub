# Сравнение правил и Machine Learning

Исследование находится в независимом модуле `ds/experiments/`.
Из каталога `ds` с активированным отдельным окружением:

```bash
python -m experiments.run_experiment
```

[Полная инструкция](../../ds/experiments/README.md).
[Зафиксированные результаты](SCORING_BENCHMARK_RESULTS.md).

6000 заявок на набор, seed 42/43/44, сценарии base/high_noise/stress;
разбиение 70/15/15. Сравниваются действующий порог score >= 70 и Logistic Regression
с порогом по стоимости FP/FN на validation. Чистая доходность — 8%, LGD — 45%.
Исходные REJECT/MANUAL/AUTO_APPROVE показаны отдельно, ручная проверка не моделируется.

Все данные и исходы синтетические. ML не подключается к рабочему API и решениям MVP
