"""
utils.py — low-entropy utility module with minimal complexity.
This should score MUCH lower than data_pipeline.py.
"""


def add(a: float, b: float) -> float:
    """Pure arithmetic — no allocations, no I/O."""
    return a + b


def clamp(value: float, lo: float, hi: float) -> float:
    """Clamp value to [lo, hi] — single branch."""
    if value < lo:
        return lo
    if value > hi:
        return hi
    return value


def format_label(name: str, score: float) -> str:
    """Format a label string — pure string manipulation."""
    return f"{name}: {score:.2f}"


CONSTANTS = {
    "version": "1.0.0",
    "max_retries": 3,
}
