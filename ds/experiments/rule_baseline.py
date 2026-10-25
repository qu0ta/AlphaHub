import numpy as np

from scoring import MODEL_VERSION, score_application


def evaluate_rules(rows):
    # Invoke the actual MVP implementation. No duplicate scoring formulas.
    scores = np.asarray([score_application(row.scoring_input).score for row in rows], dtype=int)
    bands = np.where(scores < 40, "REJECT", np.where(scores < 70, "MANUAL", "AUTO_APPROVE"))
    distribution = {name: {"count": int(np.sum(bands == name)),
                           "defaults": int(sum(row.default for row, band in zip(rows, bands) if band == name))}
                    for name in ("REJECT", "MANUAL", "AUTO_APPROVE")}
    return {"scores": scores, "predicted_default": (scores < 70).astype(int),
            "risk_ranking": 1 - scores / 100, "bands": bands,
            "three_level_distribution": distribution, "version": MODEL_VERSION}
