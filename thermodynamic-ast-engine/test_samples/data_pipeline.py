"""
data_pipeline.py — a deliberately "hot" Python module for testing the
Thermodynamic AST Engine. Contains deep nesting, blocking I/O, allocations,
and high cyclomatic complexity.
"""

import time
import threading
import requests
import numpy as np


# ── Module-level shared lock (SyncContention signal) ────────────────────────
_cache_lock = threading.Lock()
_result_cache = {}


def fetch_user_data(user_ids: list) -> dict:
    """Fetches user records from a remote API — blocking I/O inside a loop."""
    results = {}

    for uid in user_ids:                              # loop ×1
        for attempt in range(3):                      # loop ×2 — deep nesting
            try:
                response = requests.get(             # BlockingIO
                    f"https://api.example.com/users/{uid}",
                    timeout=5
                )
                if response.status_code == 200:
                    results[uid] = response.json()
                    break
                elif response.status_code == 429:
                    time.sleep(2 ** attempt)          # BlockingIO inside nested loop
            except Exception as e:
                if attempt == 2:
                    results[uid] = None

    return results


def build_feature_matrix(records: list) -> np.ndarray:
    """Allocates large numpy arrays inside nested loops — HotAllocation signals."""
    n = len(records)
    matrix = np.zeros((n, 512))                       # allocation

    for i, record in enumerate(records):              # loop ×1
        features = np.zeros(512)                      # allocation inside loop
        for j, field in enumerate(record.get("fields", [])):   # loop ×2
            if field and isinstance(field, (int, float)):
                if field > 0:
                    if j < 512:
                        features[j] = float(field) / 255.0  # deep nesting
        matrix[i] = features

    return matrix


def recursive_flatten(data, depth=0):
    """Recursive flattening — RecursiveCall signal."""
    result = []
    if depth > 50:
        return result

    if isinstance(data, list):
        for item in data:                             # loop inside recursion
            if isinstance(item, list):
                result.extend(recursive_flatten(item, depth + 1))  # recursive call
            else:
                result.append(item)
    return result


class DataProcessor:
    def __init__(self, source_url: str):
        self.source_url = source_url
        self._lock = threading.RLock()               # SyncContention
        self._buffer = []

    def process_batch(self, batch_ids: list) -> list:
        """Heavy method: I/O + allocations + branching + sync."""
        import io
        outputs = []

        with self._lock:                              # SyncContention in loop context
            for bid in batch_ids:                    # loop ×1
                raw = requests.get(                  # BlockingIO
                    f"{self.source_url}/batch/{bid}", timeout=10
                )
                if not raw or raw.status_code != 200:
                    continue
                elif raw.status_code == 503:
                    time.sleep(5)
                    continue

                records = raw.json().get("records", [])
                chunk = list(records)                # allocation
                local_buf = dict()                   # allocation

                for rec in chunk:                    # loop ×2
                    for key, val in rec.items():     # loop ×3 — triple nesting
                        if val and key and not key.startswith("_"):
                            if isinstance(val, str) and len(val) > 0:
                                local_buf[key] = val.strip()

                outputs.append(local_buf)

        return outputs
