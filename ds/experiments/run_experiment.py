"""Run the complete synthetic scoring study from ds: python -m experiments.run_experiment."""

import argparse
import csv
import hashlib
import json
import platform
from pathlib import Path

import numpy as np
import sklearn

from .config import ExperimentConfig
from .evaluate import evaluate_dataset
from .features import FEATURE_NAMES
from .synthetic_dataset import describe_dataset, generate_dataset, save_dataset
from .train_model import save_model, split_dataset, train_logistic


def write_csv(path: Path, rows):
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", newline="", encoding="utf-8") as stream:
        writer = csv.DictWriter(stream, fieldnames=list(rows[0]))
        writer.writeheader()
        writer.writerows(rows)


def aggregate_runs(runs):
    """Sample standard deviation across fixed seeds, never select a winning seed."""
    summary = {}
    for scenario in ("base", "high_noise", "stress"):
        summary[scenario] = {}
        for algorithm in ("rules", "logistic_regression"):
            values = {name: [run["scenarios"][scenario]["metrics"][algorithm][name] for run in runs]
                      for name in ("accuracy", "precision", "recall", "f1", "roc_auc", "pr_auc",
                                   "false_positive_rate", "false_negative_rate", "approval_rate")}
            values["cost_per_application_kopecks"] = [
                run["scenarios"][scenario]["financials"][algorithm]["cost_per_application_kopecks"] for run in runs
            ]
            summary[scenario][algorithm] = {
                name: {"mean": float(np.mean(observations)), "std": float(np.std(observations, ddof=1)),
                       "min": float(np.min(observations)), "max": float(np.max(observations))}
                if all(value is not None for value in observations) else None
                for name, observations in values.items()
            }
        effects = [run["scenarios"][scenario]["financial_comparison"]["relative_loss_reduction_percent"] for run in runs]
        summary[scenario]["relative_loss_reduction_percent"] = (
            {"mean": float(np.mean(effects)), "std": float(np.std(effects, ddof=1))}
            if all(value is not None for value in effects) else None
        )
    return summary


def run_study(config: ExperimentConfig, output: Path):
    data_dir, models_dir, results_dir = (output / name for name in ("data", "models", "results"))
    for directory in (data_dir, models_dir, results_dir):
        directory.mkdir(parents=True, exist_ok=True)
    manifest = {"source": "synthetic_only", "config": config.to_dict(), "feature_names": FEATURE_NAMES,
                "versions": {"python": platform.python_version(), "numpy": np.__version__,
                             "scikit_learn": sklearn.__version__}, "runs": []}
    all_predictions, financial_rows, splits_rows = [], [], []
    primary_rows, primary_predictions, primary_curve = None, None, None
    for seed_index, seed in enumerate(config.seeds):
        print(f"Seed {seed}: генерация базового набора и обучение", flush=True)
        base = generate_dataset(config.samples, seed, "base", config.scenarios["base"], f"seed{seed}-base")
        base_path = data_dir / ("synthetic_clients.csv" if seed_index == 0 else f"seed_{seed}_base.csv")
        save_dataset(base, base_path)
        splits, split_summary = split_dataset(base, seed)
        partitions = {name: [base[index] for index in indices] for name, indices in splits.items()}
        pipeline, threshold, curve = train_logistic(partitions["train"], partitions["validation"], seed, config.economics)
        model_name = "logistic_regression.joblib" if seed_index == 0 else f"logistic_regression_seed_{seed}.joblib"
        save_model(pipeline, threshold, models_dir / model_name, config.to_dict(), seed)
        run = {"seed": seed, "split": split_summary, "ml_threshold": threshold,
               "validation_threshold_curve": curve, "datasets": {}, "scenarios": {},
               "model": f"models/{model_name}",
               "standardized_coefficients": dict(zip(FEATURE_NAMES, pipeline[-1].coef_[0].tolist()))}
        for name, indices in splits.items():
            splits_rows.extend({"seed": seed, "application_id": base[index].application_id,
                                "inn": base[index].scoring_input.inn, "split": name} for index in indices)
        fit_clients = {row.scoring_input.inn for row in partitions["train"] + partitions["validation"]}
        for scenario_name in config.scenarios:
            if scenario_name == "base":
                generated, evaluation = base, partitions["test"]
                path, generation_seed = base_path, seed
            else:
                print(f"Seed {seed}: независимая проверка {scenario_name}", flush=True)
                # Separate datasets, RNG seeds and client IDs; never fit or tune on these.
                generation_seed = seed + (100_000 if scenario_name == "high_noise" else 200_000)
                generated = generate_dataset(config.samples, generation_seed, scenario_name,
                                             config.scenarios[scenario_name], f"seed{seed}-{scenario_name}")
                evaluation = generated
                path = data_dir / f"seed_{seed}_{scenario_name}.csv"
                save_dataset(generated, path)
            if fit_clients & {row.scoring_input.inn for row in evaluation}:
                raise ValueError("Клиенты обучения/валидации попали в оценочную выборку")
            if {row.default for row in evaluation} != {0, 1}:
                raise ValueError(f"В оценочном наборе {scenario_name} нужны оба класса")
            summary = describe_dataset(generated)
            summary.update({"generation_seed": generation_seed, "path": str(path.relative_to(output)),
                            "sha256": hashlib.sha256(path.read_bytes()).hexdigest()})
            run["datasets"][scenario_name] = summary
            result, predictions = evaluate_dataset(evaluation, pipeline, threshold, config.economics)
            run["scenarios"][scenario_name] = result
            all_predictions.extend({"seed": seed, **row} for row in predictions)
            for algorithm, financial in result["financials"].items():
                financial_rows.append({"seed": seed, "scenario": scenario_name, "algorithm": algorithm,
                                       "count": result["count"], **financial, **result["financial_comparison"]})
            if seed_index == 0 and scenario_name == "base":
                primary_rows, primary_predictions, primary_curve = evaluation, predictions, curve
        manifest["runs"].append(run)
    manifest["aggregate"] = aggregate_runs(manifest["runs"])
    write_csv(results_dir / "predictions.csv", all_predictions)
    write_csv(results_dir / "financial_comparison.csv", financial_rows)
    write_csv(results_dir / "splits.csv", splits_rows)
    (results_dir / "metrics.json").write_text(
        json.dumps(manifest, ensure_ascii=False, indent=2, allow_nan=False) + "\n", encoding="utf-8")
    from .visualize import create_figures
    from .report import write_report
    figures = create_figures(manifest, primary_rows, primary_predictions, primary_curve, results_dir)
    write_report(manifest, figures, results_dir)
    print(f"Готово: {results_dir.resolve() / 'report.md'}", flush=True)
    return manifest


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", type=Path, help="JSON с параметрами генерации и экономики")
    parser.add_argument("--output", type=Path, default=Path(__file__).resolve().parent)
    args = parser.parse_args()
    try:
        config = ExperimentConfig.from_file(args.config) if args.config else ExperimentConfig()
        run_study(config, args.output)
    except (ValueError, TypeError) as exc:
        parser.error(str(exc))


if __name__ == "__main__":
    main()
