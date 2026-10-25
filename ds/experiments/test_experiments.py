import csv
import json
import random
import tempfile
import unittest
from pathlib import Path

import joblib
import numpy as np
from pydantic import ValidationError

from schemas import ScoringInput, get_last_six_months
from scoring import score_application

from .config import Economics, ExperimentConfig, Scenario, default_scenarios
from .evaluate import classification_metrics, evaluate_dataset
from .features import FEATURE_NAMES, extract_features, feature_matrix
from .financial_effect import calculate_financials, compare_financials, tune_threshold
from .run_experiment import aggregate_runs
from .synthetic_dataset import SyntheticApplication, generate_dataset, sample_default, save_dataset
from .train_model import save_model, split_dataset, train_logistic


class ExperimentTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.rows = generate_dataset(1600, 42, "base", Scenario(), "unit-base")

    def test_config_requires_full_study_and_valid_economics(self):
        for payload in ({"samples": 4999}, {"seeds": (42, 43)}, {"seeds": (42, 42, 43)}):
            with self.assertRaises(ValueError):
                ExperimentConfig(**payload)
        with self.assertRaises(ValueError):
            Economics(lgd=1.1)
        with self.assertRaises(ValueError):
            Scenario(observation_noise=-0.1)
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / "config.json"
            path.write_text(json.dumps({"samples": 5000, "scenarios": {"high_noise": {"observation_noise": 0.35}}}))
            config = ExperimentConfig.from_file(path)
            self.assertEqual(config.scenarios["high_noise"].observation_noise, 0.35)
            self.assertEqual(config.scenarios["stress"], default_scenarios()["stress"])

    def test_valid_reproducible_financial_history_and_integer_labels(self):
        first = generate_dataset(10, 7, "base", Scenario(), "check")
        self.assertEqual(first, generate_dataset(10, 7, "base", Scenario(), "check"))
        self.assertEqual(len({row.application_id for row in self.rows}), len(self.rows))
        self.assertEqual(len({row.scoring_input.inn for row in self.rows}), len(self.rows))
        for row in self.rows:
            self.assertEqual([month.period_month for month in row.scoring_input.financials],
                             get_last_six_months(row.scoring_input.application_date))
        for value in (True, "1", 2, -1):
            with self.assertRaises(ValidationError):
                SyntheticApplication(**{**first[0].model_dump(), "default": value})

    def test_default_is_stochastic_and_not_determined_by_rule_score(self):
        data = self.rows[0].scoring_input
        outcomes = {sample_default(data, random.Random(seed), Scenario()) for seed in range(200)}
        self.assertEqual(outcomes, {0, 1})
        by_score = {}
        for row in self.rows:
            by_score.setdefault(score_application(row.scoring_input).score, set()).add(row.default)
        self.assertTrue(any(labels == {0, 1} for labels in by_score.values()))
        self.assertTrue(0.10 <= np.mean([row.default for row in self.rows]) <= 0.25)

    def test_feature_whitelist_excludes_targets_profiles_and_scenarios(self):
        row = self.rows[0]
        changed = row.model_copy(update={"default": 1 - row.default, "profile": "risky", "scenario": "stress"})
        np.testing.assert_array_equal(feature_matrix([row]), feature_matrix([changed]))
        self.assertTrue(all(name not in FEATURE_NAMES for name in ("default", "profile", "scenario", "inn")))
        self.assertEqual(len(extract_features(row.scoring_input)), 12)

    def test_csv_retains_complete_scoring_input(self):
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / "clients.csv"
            save_dataset(self.rows[:5], path)
            with path.open() as stream:
                restored = list(csv.DictReader(stream))
            self.assertEqual(len(restored), 5)
            self.assertEqual(ScoringInput.model_validate_json(restored[0]["scoring_input_json"]),
                             self.rows[0].scoring_input)

    def test_stratified_70_15_15_split_is_complete_and_reproducible(self):
        splits, summary = split_dataset(self.rows, 42)
        repeated, _ = split_dataset(self.rows, 42)
        self.assertEqual([len(part) for part in splits.values()], [1120, 240, 240])
        self.assertEqual(len(set(np.concatenate(list(splits.values())))), len(self.rows))
        global_rate = np.mean([row.default for row in self.rows])
        for name in splits:
            np.testing.assert_array_equal(splits[name], repeated[name])
            self.assertLess(abs(summary["sets"][name]["default_rate"] - global_rate), 0.01)

    def test_repeated_clients_stay_in_one_split(self):
        grouped = [row.model_copy(update={"scoring_input": row.scoring_input.model_copy(
            update={"inn": f"company-{index // 2}"})}) for index, row in enumerate(self.rows)]
        splits, _ = split_dataset(grouped, 42)
        owners = {}
        for name, indices in splits.items():
            for index in indices:
                inn = grouped[index].scoring_input.inn
                self.assertEqual(owners.setdefault(inn, name), name)
        self.assertEqual(len(owners), 800)

    def test_confusion_matrix_and_all_required_metrics(self):
        metrics = classification_metrics(np.array([0, 0, 1, 1]), np.array([0, 1, 0, 1]),
                                         np.array([0.1, 0.8, 0.2, 0.9]), probability=True)
        self.assertEqual(metrics["confusion_matrix"], {"tn": 1, "fp": 1, "fn": 1, "tp": 1})
        for name in ("accuracy", "precision", "recall", "f1", "false_positive_rate", "false_negative_rate"):
            self.assertEqual(metrics[name], 0.5)
        self.assertIsNotNone(metrics["roc_auc"])
        self.assertIsNotNone(metrics["pr_auc"])

    def test_financial_cost_matches_specification_and_excludes_manual_review(self):
        y, predicted = np.array([0, 0, 1, 1]), np.array([0, 1, 0, 1])
        amounts = np.array([100_000, 200_000, 300_000, 400_000])
        financials = calculate_financials(y, predicted, amounts, Economics())
        self.assertEqual(financials["fn_default_loss_kopecks"], 135_000)
        self.assertEqual(financials["fp_opportunity_loss_kopecks"], 16_000)
        self.assertEqual(financials["total_error_cost_kopecks"], 151_000)
        self.assertEqual(financials["approved_portfolio_profit_kopecks"], -127_000)
        self.assertEqual(financials["manual_review_cost_applied_kopecks"], 0)
        self.assertEqual(compare_financials(financials, {"total_error_cost_kopecks": 75_500}),
                         {"ml_effect_kopecks": 75_500, "relative_loss_reduction_percent": 50.0})
        self.assertIsNone(compare_financials({"total_error_cost_kopecks": 0},
                                           {"total_error_cost_kopecks": 0})["relative_loss_reduction_percent"])

    def test_threshold_search_matches_brute_force_with_ties(self):
        y = np.array([0, 1, 0, 1, 0, 1])
        probabilities = np.array([0.1, 0.2, 0.2, 0.8, 0.9, 0.9])
        amounts = np.array([10_000, 400_000, 50_000, 90_000, 200_000, 80_000])
        threshold, curve = tune_threshold(y, probabilities, amounts, Economics())
        brute = []
        for point in curve:
            predicted = (probabilities >= point["threshold"]).astype(int)
            cost = calculate_financials(y, predicted, amounts, Economics())["total_error_cost_kopecks"]
            self.assertEqual(cost, point["cost_kopecks"])
            brute.append((cost, int(np.sum(predicted == 0)), point["threshold"]))
        self.assertEqual(threshold, min(brute)[2])

    def test_only_train_fits_preprocessing_and_test_labels_cannot_change_predictions(self):
        splits, _ = split_dataset(self.rows, 42)
        parts = {name: [self.rows[index] for index in indices] for name, indices in splits.items()}
        pipeline, threshold, _ = train_logistic(parts["train"], parts["validation"], 42, Economics())
        np.testing.assert_allclose(pipeline[0].statistics_, np.nanmedian(feature_matrix(parts["train"]), axis=0))
        snapshot = pipeline[-1].coef_.copy()
        first, predictions = evaluate_dataset(parts["test"], pipeline, threshold, Economics())
        flipped = [row.model_copy(update={"default": 1 - row.default}) for row in parts["test"]]
        second, changed_predictions = evaluate_dataset(flipped, pipeline, threshold, Economics())
        self.assertEqual([row["ml_probability"] for row in predictions],
                         [row["ml_probability"] for row in changed_predictions])
        self.assertNotEqual(first["metrics"]["logistic_regression"]["confusion_matrix"],
                            second["metrics"]["logistic_regression"]["confusion_matrix"])
        np.testing.assert_array_equal(snapshot, pipeline[-1].coef_)
        self.assertEqual(sum(item["count"] for item in first["three_level_rule_decisions"].values()), len(parts["test"]))
        self.assertEqual([row["rule_score"] for row in predictions],
                         [score_application(row.scoring_input).score for row in parts["test"]])

    def test_saved_pipeline_reproduces_pd_and_separate_stress_set_never_refits(self):
        splits, _ = split_dataset(self.rows, 42)
        train, validation = ([self.rows[index] for index in splits[name]] for name in ("train", "validation"))
        pipeline, threshold, _ = train_logistic(train, validation, 42, Economics())
        stress = generate_dataset(200, 200_042, "stress", default_scenarios()["stress"], "unit-stress")
        self.assertFalse({row.scoring_input.inn for row in self.rows} & {row.scoring_input.inn for row in stress})
        coefficients, scale = pipeline[-1].coef_.copy(), pipeline[1].mean_.copy()
        evaluate_dataset(stress, pipeline, threshold, Economics())
        np.testing.assert_array_equal(coefficients, pipeline[-1].coef_)
        np.testing.assert_array_equal(scale, pipeline[1].mean_)
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / "model.joblib"
            save_model(pipeline, threshold, path, ExperimentConfig().to_dict(), 42)
            restored = joblib.load(path)  # Only the artifact created in this test.
            self.assertEqual(restored["threshold"], threshold)
            np.testing.assert_allclose(restored["pipeline"].predict_proba(feature_matrix(stress)),
                                       pipeline.predict_proba(feature_matrix(stress)))

    def test_seed_aggregation_reports_sample_standard_deviation(self):
        runs = []
        for value in (0.1, 0.2, 0.3):
            result = {"metrics": {name: {key: value for key in
                       ("accuracy", "precision", "recall", "f1", "roc_auc", "pr_auc",
                        "false_positive_rate", "false_negative_rate", "approval_rate")}
                       for name in ("rules", "logistic_regression")},
                      "financials": {name: {"cost_per_application_kopecks": value * 100}
                                     for name in ("rules", "logistic_regression")},
                      "financial_comparison": {"relative_loss_reduction_percent": value}}
            runs.append({"scenarios": {name: result for name in ("base", "high_noise", "stress")}})
        summary = aggregate_runs(runs)["base"]["rules"]["roc_auc"]
        self.assertAlmostEqual(summary["mean"], 0.2)
        self.assertAlmostEqual(summary["std"], 0.1)


if __name__ == "__main__":
    unittest.main()
