# 🔥 Thermodynamic AST Engine

A **zero-dependency heuristic entropy analyzer** for Go and Python source code.
It calculates a *thermodynamic entropy score* for every function and block in your
codebase — a weighted proxy for cyclomatic complexity, nested-loop depth, heap
allocation rate, and blocking I/O pressure — and writes a structured JSON report
of the highest-risk scaling bottlenecks.

---

## Architecture

```
thermodynamic-ast-engine/
├── Cargo.toml                 # dependencies & build profiles
├── README.md                  # this file
├── src/
│   └── main.rs                # all 12 modules, unit tests
└── test_samples/
    ├── crawler.go             # high-entropy Go target
    ├── data_pipeline.py       # high-entropy Python target
    └── utils.py               # low-entropy baseline
```

### Module map (`src/main.rs`)

| Module | Responsibility |
|--------|----------------|
| `Cli` | clap v4 argument parsing |
| Data model | `Hotspot`, `FileReport`, `ThermodynamicReport` structs |
| Regex registry | `once_cell::Lazy` patterns, compiled once at first use |
| Rule tables | `python_rules()` / `go_rules()` — per-language pattern sets |
| Language detection | Extension -> language + rule set mapping |
| Line analyzer | `analyze_line()` — applies rules, nesting multiplier |
| File scanner | `scan_file()` — buffered I/O, line iteration, aggregation |
| Directory walker | `scan_directory()` — WalkDir + Rayon parallel dispatch |
| Report builder | `build_and_write_report()` — sort, aggregate, JSON emit |
| Terminal banner | `print_summary()` — top-5 hotspot table |
| `main()` | Orchestration, validation, exit codes |
| Unit tests | 6 focused tests covering detection, clamping, multiplier |

---

## Entropy Score Model

Each pattern match starts with a **base score** and is multiplied by the
**nesting-depth multiplier**:

```
entropy = base_score x (1 + 0.5 x loop_depth^2)   clamped to [0, 100]
```

| Loop depth | Multiplier | Intuition |
|:----------:|:----------:|-----------|
| 0 | x1.0 | flat code |
| 1 | x1.5 | single loop |
| 2 | x3.0 | O(n^2) risk |
| 3 | x5.5 | O(n^3) — near-certain bottleneck |

### Vulnerability types & base scores

| Type | Base | Triggers |
|------|-----:|---------|
| `SyncContention` | 22 | `sync.Mutex`, `threading.Lock`, `atomic.*` |
| `RecursiveCall` | 20 | recursive function calls |
| `BlockingIO` | 18 | `requests.get`, `os.Open`, `time.Sleep`, `http.Get` |
| `DeepNesting` | 15 | `for`, `while` loop constructs |
| `HotAllocation` | 12 | `make(`, `new(`, `np.zeros`, `[]T{` |
| `CognitiveBranch` | 5 | `if`, `elif`, `&&`, `||`, `switch/case` |

---

## Build

### Prerequisites

- **Rust >= 1.75** — install via https://rustup.rs

```powershell
# Install Rust (one-time)
winget install Rustlang.Rustup
# Reload shell, then verify:
cargo --version
```

### Debug build (fast compile)

```powershell
cd thermodynamic-ast-engine
cargo build
```

### Release build (maximum runtime performance)

```powershell
cargo build --release
# Binary: target\release\thermodynamic-ast-engine.exe
```

---

## Run

```powershell
# Basic scan of the bundled test samples
cargo run -- .\test_samples\

# Release binary with custom output path
.\target\release\thermodynamic-ast-engine.exe `
    .\test_samples\ `
    --output report.json `
    --min-score 10 `
    --verbose

# Disable parallel scanning (single-threaded)
cargo run --no-default-features -- .\test_samples\
```

### CLI reference

```
Usage: thermodynamic-ast-engine.exe [OPTIONS] <DIR>

Arguments:
  <DIR>  Root directory to scan (recursively)

Options:
  -o, --output <FILE>      Output JSON file [default: thermodynamic_report.json]
  -m, --min-score <SCORE>  Minimum entropy score to include [default: 0]
  -v, --verbose            Show per-file progress
  -h, --help               Print help
  -V, --version            Print version
```

---

## Run unit tests

```powershell
cargo test
cargo test -- --nocapture
```

---

## JSON Report Schema

```jsonc
{
  "engine_version": "0.1.0",
  "generated_at":   "2026-08-12T17:00:00Z",
  "scanned_directory": "C:\\...\\test_samples",
  "files_analyzed":    3,
  "total_hotspots":    47,
  "global_entropy":    1284.50,

  "file_reports": [
    {
      "file_path":            "...\\data_pipeline.py",
      "language":             "Python",
      "lines_scanned":        82,
      "total_entropy":        643.25,
      "mean_hotspot_entropy": 21.44,

      "hotspots": [
        {
          "function_name":      "process_batch",
          "line_number":        71,
          "source_snippet":     "for key, val in rec.items():",
          "entropy_score":      99.0,
          "vulnerability_type": "BlockingIO",
          "description":        "Blocking I/O call on critical path"
        }
      ]
    }
  ]
}
```

---

## Extending the Engine

### Add a new language (e.g., TypeScript)

1. Add `"ts" | "tsx"` to `detect_language()`.
2. Define `static TS_*: Lazy<Regex>` patterns.
3. Implement `typescript_rules() -> Vec<PatternRule>`.
4. Map `"TypeScript"` in `func_regex_for()`.

### Add a new vulnerability type

1. Add a variant to `VulnerabilityType`.
2. Add `impl Display` arm.
3. Push a new `PatternRule` into the relevant `*_rules()` function.

---

## Performance Notes

- **Rayon** parallelises file scanning across all CPU cores.
- **`once_cell::Lazy`** compiles each regex exactly once with no locking overhead.
- **Buffered I/O** (`BufReader`) avoids loading entire files into memory.
- Release build enables **LTO + single codegen unit** for maximum throughput.
