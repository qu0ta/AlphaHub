from pathlib import Path

import joblib
import numpy as np
from sklearn.impute import SimpleImputer
from sklearn.linear_model import LogisticRegression
from sklearn.model_selection import StratifiedGroupKFold, train_test_split
from sklearn.pipeline import make_pipeline
from sklearn.preprocessing import StandardScaler

from .config import Economics
from .features import FEATURE_NAMES, feature_matrix
from .financial_effect import tune_threshold


def split_dataset(rows, seed: int):
    if len({row.application_id for row in rows}) != len(rows):
        raise ValueError("application_id должен быть уникальным")
    y = np.asarray([row.default for row in rows], dtype=int)
    groups = np.asarray([row.scoring_input.inn for row in rows])
    indices = np.arange(len(rows))
    if len(np.unique(groups)) == len(rows):
        train, rest = train_test_split(indices, train_size=0.70, stratify=y, random_state=seed)
        validation, test = train_test_split(rest, test_size=0.50, stratify=y[rest], random_state=seed)
        method = "stratified_shuffle_70_15_15"
    else:
        if len(np.unique(groups)) < 20 or min(np.bincount(y, minlength=2)) < 20:
            raise ValueError("Для группового разделения нужны >=20 клиентов и >=20 наблюдений каждого класса")
        folds = [part for _, part in StratifiedGroupKFold(
            n_splits=20, shuffle=True, random_state=seed
        ).split(indices, y, groups)]
        train, validation, test = np.concatenate(folds[:14]), np.concatenate(folds[14:17]), np.concatenate(folds[17:])
        method = "stratified_group_14_3_3_folds"
    splits = {"train": np.sort(train), "validation": np.sort(validation), "test": np.sort(test)}
    for name, part in splits.items():
        if len(part) == 0 or set(y[part]) != {0, 1}:
            raise ValueError(f"В выборке {name} должны присутствовать оба класса")
    company_sets = [set(groups[part]) for part in splits.values()]
    if any(company_sets[a] & company_sets[b] for a, b in ((0, 1), (0, 2), (1, 2))):
        raise ValueError("Клиенты пересекаются между выборками")
    summary = {"method": method, "sets": {
        name: {"count": len(part), "share": len(part) / len(rows),
               "default_rate": float(np.mean(y[part])), "clients": len(set(groups[part]))}
        for name, part in splits.items()
    }}
    return splits, summary


def train_logistic(train_rows, validation_rows, seed: int, economics: Economics):
    pipeline = make_pipeline(
        SimpleImputer(strategy="median", keep_empty_features=True), StandardScaler(),
        LogisticRegression(C=1.0, max_iter=2000, random_state=seed),
    )
    pipeline.fit(feature_matrix(train_rows), [row.default for row in train_rows])
    probabilities = pipeline.predict_proba(feature_matrix(validation_rows))[:, 1]
    threshold, curve = tune_threshold(
        np.asarray([row.default for row in validation_rows]), probabilities,
        np.asarray([row.scoring_input.amount_kopecks for row in validation_rows], dtype=np.int64), economics,
    )
    return pipeline, threshold, curve


def save_model(pipeline, threshold: float, path: Path, config: dict, seed: int):
    path.parent.mkdir(parents=True, exist_ok=True)
    joblib.dump({"pipeline": pipeline, "threshold": threshold, "feature_names": FEATURE_NAMES,
                 "training_source": "synthetic_base", "seed": seed,
                 "config": config, "usage": "offline_research_only"}, path)
