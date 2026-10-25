from .features import FEATURE_NAMES
from .visualize import ALGORITHMS, LABELS, SCENARIOS


def rub(kopecks):
    return f"{kopecks / 100:,.2f}".replace(",", " ")


def pct(value):
    return "—" if value is None else f"{100 * value:.2f}%"


def number(value):
    return "—" if value is None else f"{value:.4f}"


def write_report(manifest, figures, output):
    primary = manifest["runs"][0]
    base = primary["scenarios"]["base"]
    economics = manifest["config"]["economics"]
    lines = [
        "# Исследование эффективности кредитного скоринга: правила и ML", "",
        "**Все данные и исходы синтетические. Результаты описывают моделируемые условия "
        "и не доказывают эффективность на реальных банковских данных.**", "",
        "## 1. Цель и границы", "",
        "Сравнить существующий financial_rules_v1 и Logistic Regression по выявлению дефолтов, "
        "ошибкам I/II рода, условной стоимости ошибок и устойчивости к шуму и стрессу. "
        "Эксперимент изолирован от MVP; ML не подключается к кредитному решению.", "",
        "## 2. Синтетические данные и целевая переменная", "",
        f"На каждый seed ({', '.join(map(str, manifest['config']['seeds']))}) создано по "
        f"{manifest['config']['samples']} уникальных заявок для каждого из трёх сценариев. "
        "У заявки шесть полных календарных месяцев перед датой подачи, целые копейки "
        "и условия, валидные для ScoringInput. Горизонт исхода — 12 месяцев.", "",
        "Профили stable/medium/risky задают перекрывающиеся распределения тренда, волатильности, "
        "расходов и действующего платежа. Добавлены сезонность, ежемесячные колебания, "
        "разовые положительные и отрицательные шоки и мультипликативный шум наблюдений. "
        "Профиль не передаётся модели.", "",
        "default семплируется из Bernoulli по отдельной логистической функции латентных финансов: "
        "покрытие платежей, денежная маржа, CV, динамика, долговая нагрузка и срок. "
        "Есть нелинейное взаимодействие низкой маржи с высоким долгом, ненаблюдаемый будущий шок "
        "и скрытая случайная компонента. Итоговый score правил не используется. "
        "Истинная PD и скрытые факторы не сохраняются в обучающие данные.", "",
        "| Seed | Сценарий | Заявок | Дефолты |", "|---:|---|---:|---:|",
    ]
    for run in manifest["runs"]:
        for scenario, dataset in run["datasets"].items():
            label = "Базовый набор до разбиения" if scenario == "base" else SCENARIOS[scenario]
            lines.append(f"| {run['seed']} | {label} | {dataset['count']} | {pct(dataset['default_rate'])} |")
    lines += ["", "Целевая доля дефолтов base — ориентировочно 10–25%. "
              "В metrics.json сохранены фактическая проверка диапазона, число уникальных клиентов, "
              "численность профилей и p05/медиана/p95 признаков для каждого сгенерированного набора.", "",
              "## 3. Признаки", "",
              "Используются только показатели из ScoringInput на момент заявки. Расчёты "
              "переиспользуют metrics.py. Значения default, профиль, сценарий, ИНН, идентификатор "
              "и любые будущие события отсутствуют в матрице признаков.", "",
              ", ".join(FEATURE_NAMES) + ".", "",
              "## 4. Алгоритмы, обучение и порог", "",
              "Правила: вызывается реальный scoring.score_application; score ≥ 70 означает "
              "автоматическое одобрение, score < 70 — отсутствие автоматического одобрения. "
              "Ранжирование риска для ROC/PR использует 1 − score/100; эта величина не считается PD.", "",
              "ML: SimpleImputer(median) → StandardScaler → LogisticRegression(C=1, max_iter=2000). "
              "Препроцессинг и регрессия обучены только на train. Разбиение 70/15/15 "
              "стратифицировано; при повторных клиентах используется групповое разделение "
              "с приблизительными пропорциями 70/15/15. Клиенты между выборками не пересекаются. "
              "Это случайное разбиение синтетики, а не проверка обобщения на будущие реальные периоды.", "",
              "Порог PD минимизирует Cost = Σ(FN_amount × LGD) + Σ(FP_amount × net_yield) "
              "на validation. Равенство PD порогу означает прогноз дефолта/отказ. "
              "Test, high_noise и stress не используются для обучения, выбора признаков или порога. "
              "Дополнительный HistGradientBoosting не включён: это необязательная часть ТЗ.", "",
              "| Seed | Train | Validation | Test | Порог PD |", "|---:|---:|---:|---:|---:|"]
    for run in manifest["runs"]:
        counts = [run["split"]["sets"][name]["count"] for name in ("train", "validation", "test")]
        lines.append(f"| {run['seed']} | {counts[0]} | {counts[1]} | {counts[2]} | {run['ml_threshold']:.6f} |")
    lines += ["", "## 5. Метрики на независимом базовом test", "",
              f"Подробная таблица для первого seed={primary['seed']}, n={base['count']}. "
              "Ниже также приведены все seed и их среднее с разбросом. Положительный класс — дефолт.", "",
              "| Метод | Accuracy | Precision | Recall | F1 | ROC-AUC | PR-AUC (AP) | FPR | FNR |",
              "|---|---:|---:|---:|---:|---:|---:|---:|---:|"]
    for algorithm in ALGORITHMS:
        item = base["metrics"][algorithm]
        lines.append(f"| {LABELS[algorithm]} | " + " | ".join(number(item[key]) for key in
                     ("accuracy", "precision", "recall", "f1", "roc_auc", "pr_auc", "false_positive_rate", "false_negative_rate")) + " |")
    lines += ["", "При отсутствии положительных прогнозов precision и F1 принимаются равными 0. "
              "PR-AUC представлен average precision. Brier и калибровка рассчитываются только для ML.", "",
              "## 6. Ошибки I и II рода", "",
              "FP — недефолтный клиент признан рискованным, теряется возможная прибыль. "
              "FN — дефолтный клиент признан безопасным, возникает убыток по выданному кредиту. "
              "Цена FN учитывает LGD и сумму, поэтому простое сравнение количества ошибок недостаточно.", "",
              "| Метод | TN | FP: ошибка I | FN: ошибка II | TP | Одобрение |",
              "|---|---:|---:|---:|---:|---:|"]
    for algorithm in ALGORITHMS:
        item = base["metrics"][algorithm]
        cm = item["confusion_matrix"]
        lines.append(f"| {LABELS[algorithm]} | {cm['tn']} | {cm['fp']} | {cm['fn']} | {cm['tp']} | {pct(item['approval_rate'])} |")
    lines += ["", "Исходное трёхуровневое решение правил на том же test:", "",
              "| Исходное решение | Заявок | Синтетических дефолтов |", "|---|---:|---:|"]
    for band, counts in base["three_level_rule_decisions"].items():
        lines.append(f"| {band} | {counts['count']} | {counts['defaults']} |")
    lines += ["", "MANUAL соответствует score 40–69. В основном MVP это ручная проверка, "
              "а не отказ. В бинарном исследовании она входит в «нет автоматического одобрения». "
              "Её фактические решения и затраты здесь не моделируются.", "",
              "## 7. Финансовые потери и эффект", "",
              f"Условные допущения: чистая прибыль успешного кредита = {pct(economics['net_yield'])} суммы; "
              f"LGD = {pct(economics['lgd'])} суммы. Цена ручной проверки в конфигурации — "
              f"{rub(economics['manual_review_cost_kopecks'])} ₽; в бинарном эксперименте применяется **0 ₽**.", "",
              "Cost = сумма кредитов FN × LGD + сумма кредитов FP × чистая доходность. "
              "Эффект ML = Cost правил − Cost ML; положительное значение означает снижение потерь. "
              "Относительное снижение = эффект / Cost правил × 100%; при нулевой базе оно не определено. "
              "Суммы считаются в копейках, компоненты округляются ROUND_HALF_UP до копейки. "
              "Это стоимость ошибок по условной модели, а не бухгалтерский убыток MVP.", "",
              "| Метод | FN: кредитный убыток, ₽ | FP: упущенная прибыль, ₽ | Общая стоимость ошибок, ₽ |",
              "|---|---:|---:|---:|"]
    for algorithm in ALGORITHMS:
        item = base["financials"][algorithm]
        lines.append(f"| {LABELS[algorithm]} | {rub(item['fn_default_loss_kopecks'])} | "
                     f"{rub(item['fp_opportunity_loss_kopecks'])} | {rub(item['total_error_cost_kopecks'])} |")
    effect = base["financial_comparison"]
    relative = effect["relative_loss_reduction_percent"]
    lines += ["", f"Эффект ML на первом базовом test: **{rub(effect['ml_effect_kopecks'])} ₽**; "
              + (f"относительное снижение **{relative:.2f}%**." if relative is not None else "относительное снижение не определено."), "",
              "Чувствительность первого test при фиксированных решениях и пороге:", "",
              "| LGD | Доходность | Потери правил, ₽ | Потери ML, ₽ | Эффект ML, ₽ |",
              "|---:|---:|---:|---:|---:|"]
    for item in base["sensitivity_fixed_threshold"]:
        lines.append(f"| {pct(item['lgd'])} | {pct(item['net_yield'])} | {rub(item['rules_cost_kopecks'])} | "
                     f"{rub(item['ml_cost_kopecks'])} | {rub(item['ml_effect_kopecks'])} |")
    lines += ["", "## 8. Шум, стресс и устойчивость по seed", "",
              "Для каждого seed модель обучена один раз на base. High_noise и stress — "
              "дополнительные независимые наборы с другими RNG seed и идентификаторами клиентов. "
              "В high_noise усилены наблюдаемый шум, волатильность, сезонность и разовые шоки. "
              "В stress также снижены поступления, повышены относительные расходы и сдвинут скрытый риск. "
              "Для этих наборов не переобучаются даже медианы и масштабирование, порог заморожен.", "",
              "| Seed | Сценарий | ROC-AUC правила / ML | FNR правила / ML | Эффект ML, ₽ | Снижение потерь |",
              "|---:|---|---:|---:|---:|---:|"]
    for run in manifest["runs"]:
        for scenario, result in run["scenarios"].items():
            rules, ml = (result["metrics"][name] for name in ALGORITHMS)
            relative = result["financial_comparison"]["relative_loss_reduction_percent"]
            lines.append(f"| {run['seed']} | {SCENARIOS[scenario]} | {number(rules['roc_auc'])} / {number(ml['roc_auc'])} | "
                         f"{pct(rules['false_negative_rate'])} / {pct(ml['false_negative_rate'])} | "
                         f"{rub(result['financial_comparison']['ml_effect_kopecks'])} | "
                         + (f"{relative:.2f}% |" if relative is not None else "— |"))
    lines += ["", "Агрегирование: среднее ± выборочное стандартное отклонение между seed (ddof=1). "
              "Размеры base test и дополнительных наборов различаются; для сравнения сценариев "
              "потери нормированы на одну заявку. Все исходные метрики и размеры есть в metrics.json.", "",
              "| Сценарий | Метод | ROC-AUC, mean ± SD | FNR, mean ± SD | Потери на заявку, ₽, mean ± SD |",
              "|---|---|---:|---:|---:|"]
    for scenario, summary in manifest["aggregate"].items():
        for algorithm in ALGORITHMS:
            values = summary[algorithm]
            auc, fnr, cost = (values[name] for name in ("roc_auc", "false_negative_rate", "cost_per_application_kopecks"))
            lines.append(f"| {SCENARIOS[scenario]} | {LABELS[algorithm]} | {auc['mean']:.4f} ± {auc['std']:.4f} | "
                         f"{pct(fnr['mean'])} ± {pct(fnr['std'])} | {rub(cost['mean'])} ± {rub(cost['std'])} |")
    lines += ["", "## 9. Выводы по моделируемым условиям", ""]
    for scenario, summary in manifest["aggregate"].items():
        relative = summary["relative_loss_reduction_percent"]
        if relative:
            direction = "снижает" if relative["mean"] > 0 else "увеличивает"
            lines.append(f"{SCENARIOS[scenario]}: ML в среднем {direction} условную стоимость ошибок "
                         f"на {abs(relative['mean']):.2f}% (SD {relative['std']:.2f} п.п.). "
                         "Это описание выбранных seed и процесса генерации, не статистическое доказательство для банка.")
    lines += ["", "Правила просты для объяснения и не требуют размеченной истории; их фиксированные "
              "границы дают грубое ранжирование. Logistic Regression даёт непрерывную PD и позволяет "
              "выбрать порог по стоимости ошибок, но зависит от обучающего распределения, линейной "
              "формы модели и калибровки. Изменение числа FP/FN следует оценивать вместе с суммами кредитов.", "",
              "## 10. Ограничения и развитие AlphaHub", "",
              "Генератор задаёт сам мир эксперимента, поэтому его зависимости могут благоприятствовать "
              "одному методу. Нет реальных проверенных исходов, отбора выдач, коллекшена, дат дефолта, "
              "изменения EAD и реальных результатов ручного рассмотрения. Доходность и LGD постоянны. "
              "Три seed показывают разброс, но не дают полной оценки неопределённости. "
              "Порог правил фиксирован на 70, а порог ML настроен на validation: эффект включает "
              "как алгоритм, так и кредитную политику. Для их разделения нужен дополнительный "
              "контроль с настроенным порогом правил или одинаковой долей одобрения.", "",
              "Для реального выбора нужны согласованное определение дефолта и горизонт, "
              "зрелая размеченная история, временная проверка, контроль смещения одобренных кредитов, "
              "калибровки и дрейфа. Можно дополнить исследование HistGradientBoosting и отдельным "
              "анализом ручной проверки. Автоматическая замена текущего скоринга по этому отчёту не выполняется.", "",
              "## Автоматически построенные графики", ""]
    for name in figures:
        lines += [f"![{name}]({name})", ""]
    lines += ["Файлы: metrics.json — все результаты и конфигурация; predictions.csv — построчные "
              "прогнозы; financial_comparison.csv — финансы; splits.csv — принадлежность заявок "
              "к выборкам; models/*.joblib — модели и пороги по seed.", "",
              "Методические источники: "
              "[разбиение с группами](https://scikit-learn.org/stable/modules/generated/sklearn.model_selection.StratifiedGroupKFold.html), "
              "[утечка данных](https://scikit-learn.org/stable/common_pitfalls.html#data-leakage), "
              "[настройка порога](https://scikit-learn.org/stable/modules/classification_threshold.html).", ""]
    (output / "report.md").write_text("\n".join(lines), encoding="utf-8")
