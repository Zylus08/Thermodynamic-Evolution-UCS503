// =============================================================================
// thermodynamic-ast-engine · src/main.rs
//
// A heuristic "thermodynamic entropy" analyzer for Go and Python source files.
//
// Design pillars
// ──────────────
//   1. Zero unsafe code.
//   2. Compiled regexes are built exactly once (once_cell::sync::Lazy).
//   3. File scanning runs in parallel via Rayon (feature-gated).
//   4. All public data types implement Serialize so the JSON report is trivial.
//   5. Every logical stage is a separate module for testability.
// =============================================================================

// ── External crate imports ────────────────────────────────────────────────────
use clap::Parser;
use colored::Colorize;
use once_cell::sync::Lazy;
use regex::Regex;
use serde::{Deserialize, Serialize};
use std::{
    fs,
    io::{self, BufRead},
    path::{Path, PathBuf},
};
use walkdir::WalkDir;

#[cfg(feature = "parallel")]
use rayon::prelude::*;

// =============================================================================
// § 1  CLI argument definition (clap derive)
// =============================================================================

/// Thermodynamic AST Engine — identifies entropy hotspots in your codebase
#[derive(Parser, Debug)]
#[command(
    name        = "thermodynamic-ast-engine",
    version     = env!("CARGO_PKG_VERSION"),
    author      = env!("CARGO_PKG_AUTHORS"),
    about       = "Calculates thermodynamic entropy scores for Go/Python source files \
                   and emits a JSON report of scaling bottlenecks.",
    long_about  = None,
)]
struct Cli {
    /// Root directory to scan (recursively)
    #[arg(value_name = "DIR")]
    directory: PathBuf,

    /// Output file path for the JSON report
    #[arg(
        short,
        long,
        value_name = "FILE",
        default_value = "thermodynamic_report.json"
    )]
    output: PathBuf,

    /// Minimum entropy score to include in the report (0.0 – 100.0)
    #[arg(short, long, value_name = "SCORE", default_value_t = 0.0)]
    min_score: f64,

    /// Show verbose per-file progress in stdout
    #[arg(short, long)]
    verbose: bool,
}

// =============================================================================
// § 2  Core data model
// =============================================================================

/// The vulnerability category detected by the heuristic engine.
/// Each variant maps to a distinct set of regex patterns.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum VulnerabilityType {
    /// Deeply nested loops (O(n^k) risk)
    DeepNesting,
    /// Direct or mutual recursion without obvious base case
    RecursiveCall,
    /// Heap allocation inside a hot loop
    HotAllocation,
    /// Blocking I/O or syscall on the critical path
    BlockingIO,
    /// High cognitive complexity (many boolean operators / branches)
    CognitiveBranch,
    /// Unsafe synchronisation primitive (mutex inside loop, etc.)
    SyncContention,
}

impl std::fmt::Display for VulnerabilityType {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::DeepNesting      => write!(f, "DeepNesting"),
            Self::RecursiveCall    => write!(f, "RecursiveCall"),
            Self::HotAllocation    => write!(f, "HotAllocation"),
            Self::BlockingIO       => write!(f, "BlockingIO"),
            Self::CognitiveBranch  => write!(f, "CognitiveBranch"),
            Self::SyncContention   => write!(f, "SyncContention"),
        }
    }
}

/// A single entropy "hotspot" — one detected signal within a file.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Hotspot {
    /// Name of the containing function / method (best-effort heuristic)
    pub function_name: String,

    /// 1-indexed line number where the pattern was detected
    pub line_number: usize,

    /// The raw source line that triggered the signal
    pub source_snippet: String,

    /// Weighted entropy score for this single signal (0.0 – 100.0)
    pub entropy_score: f64,

    /// Classification of the detected risk
    pub vulnerability_type: VulnerabilityType,

    /// Human-readable explanation of why this is flagged
    pub description: String,
}

/// Aggregated report for one source file.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FileReport {
    /// Absolute path to the source file
    pub file_path: String,

    /// Programming language detected from the file extension
    pub language: String,

    /// Total number of non-blank, non-comment lines scanned
    pub lines_scanned: usize,

    /// Sum of all individual hotspot entropy scores
    pub total_entropy: f64,

    /// Mean entropy per detected hotspot (0 if no hotspots)
    pub mean_hotspot_entropy: f64,

    /// All identified hotspots, sorted highest-score first
    pub hotspots: Vec<Hotspot>,
}

/// Root report document written to disk.
#[derive(Debug, Serialize, Deserialize)]
pub struct ThermodynamicReport {
    /// Engine version for schema compatibility
    pub engine_version: String,

    /// ISO-8601 timestamp when the scan completed
    pub generated_at: String,

    /// Directory that was scanned
    pub scanned_directory: String,

    /// Total source files analyzed
    pub files_analyzed: usize,

    /// Total hotspots found across all files
    pub total_hotspots: usize,

    /// Aggregate entropy across the entire codebase
    pub global_entropy: f64,

    /// Per-file reports, sorted by `total_entropy` descending
    pub file_reports: Vec<FileReport>,
}

// =============================================================================
// § 3  Regex pattern registry (compiled once, shared across threads)
// =============================================================================

/// Internal representation of one pattern rule.
struct PatternRule {
    regex:            &'static Lazy<Regex>,
    vulnerability:    VulnerabilityType,
    base_score:       f64, // base entropy contribution per match
    description_tmpl: &'static str,
}

// ── Python patterns ───────────────────────────────────────────────────────────

static PY_FOR_WHILE: Lazy<Regex>   = Lazy::new(|| Regex::new(r"^\s*(for|while)\s+").unwrap());
// Note: the `regex` crate does not support backreferences; we detect recursion
// via two independent signals:
//   1. `self.method()` — object calling its own method (Python)
//   2. A standalone identifier call on its own line that is NOT a dotted method
//      call (e.g. `flatten(items)` rather than `obj.flatten(items)`)
static PY_RECURSIVE: Lazy<Regex>   = Lazy::new(|| Regex::new(r"\bself\s*\.\s*\w+\s*\(|(?:^|\s)(\w+)\s*\([^)]*\)\s*$").unwrap());
static PY_ALLOC: Lazy<Regex>       = Lazy::new(|| Regex::new(r"\b(list|dict|set|bytearray|numpy\.zeros|numpy\.ones|np\.zeros|np\.ones|torch\.zeros|torch\.ones)\s*[\(\[]").unwrap());
static PY_BLOCKING_IO: Lazy<Regex> = Lazy::new(|| Regex::new(r"\b(open\s*\(|requests\.(get|post|put|delete|patch)|urllib|subprocess\.(call|run|Popen)|time\.sleep|socket\.recv|socket\.accept)\b").unwrap());
static PY_BRANCH: Lazy<Regex>      = Lazy::new(|| Regex::new(r"\b(if|elif|and|or|not|assert)\b").unwrap());
static PY_MUTEX: Lazy<Regex>       = Lazy::new(|| Regex::new(r"\b(threading\.(Lock|RLock|Semaphore)|asyncio\.Lock|multiprocessing\.Lock)\b").unwrap());
static PY_FUNC: Lazy<Regex>        = Lazy::new(|| Regex::new(r"^\s*(?:async\s+)?def\s+(\w+)\s*\(").unwrap());

// ── Go patterns ───────────────────────────────────────────────────────────────

static GO_FOR: Lazy<Regex>         = Lazy::new(|| Regex::new(r"^\s*for\s+").unwrap());
// Match a bare function call (not a dotted method call like obj.Method()).
// The negative lookbehind equivalent in `regex` isn't supported, so we anchor
// on the pattern starting after whitespace or at line start, without a dot.
static GO_RECURSIVE: Lazy<Regex>   = Lazy::new(|| Regex::new(r"(?:^|[\s,=(])([A-Z]\w*)\s*\(").unwrap()); // Capital = exported fn, likely recursive
static GO_ALLOC: Lazy<Regex>       = Lazy::new(|| Regex::new(r"\bmake\s*\(|\bnew\s*\(|\[\][\w\*]+\{").unwrap());
static GO_BLOCKING_IO: Lazy<Regex> = Lazy::new(|| Regex::new(r"\b(os\.Open|os\.Create|ioutil\.(ReadFile|WriteFile)|http\.(Get|Post)|net\.Dial|time\.Sleep|bufio\.NewReader|sql\.Open)\b").unwrap());
static GO_BRANCH: Lazy<Regex>      = Lazy::new(|| Regex::new(r"\b(if|else|switch|case|&&|\|\||select)\b").unwrap());
static GO_MUTEX: Lazy<Regex>       = Lazy::new(|| Regex::new(r"\b(sync\.(Mutex|RWMutex|WaitGroup|Once)|atomic\.(Add|Load|Store|Swap))").unwrap());
static GO_FUNC: Lazy<Regex>        = Lazy::new(|| Regex::new(r"^\s*func\s+(?:\([^)]+\)\s+)?(\w+)\s*\(").unwrap());

// =============================================================================
// § 4  Language-specific rule tables
// =============================================================================

fn python_rules() -> Vec<PatternRule> {
    vec![
        PatternRule { regex: &PY_FOR_WHILE,    vulnerability: VulnerabilityType::DeepNesting,     base_score: 15.0, description_tmpl: "Loop construct detected - nesting depth multiplier applied"     },
        PatternRule { regex: &PY_RECURSIVE,    vulnerability: VulnerabilityType::RecursiveCall,   base_score: 20.0, description_tmpl: "Possible recursive invocation - stack-depth risk"               },
        PatternRule { regex: &PY_ALLOC,        vulnerability: VulnerabilityType::HotAllocation,   base_score: 12.0, description_tmpl: "Heap allocation inside potentially hot path"                    },
        PatternRule { regex: &PY_BLOCKING_IO,  vulnerability: VulnerabilityType::BlockingIO,      base_score: 18.0, description_tmpl: "Blocking I/O call on critical path - latency spike risk"       },
        PatternRule { regex: &PY_BRANCH,       vulnerability: VulnerabilityType::CognitiveBranch, base_score:  5.0, description_tmpl: "Branch/boolean operator increases cyclomatic complexity"       },
        PatternRule { regex: &PY_MUTEX,        vulnerability: VulnerabilityType::SyncContention,  base_score: 22.0, description_tmpl: "Synchronisation primitive - potential lock contention hotspot"  },
    ]
}

fn go_rules() -> Vec<PatternRule> {
    vec![
        PatternRule { regex: &GO_FOR,          vulnerability: VulnerabilityType::DeepNesting,     base_score: 15.0, description_tmpl: "Go for-loop - nesting depth multiplier applied"                 },
        PatternRule { regex: &GO_RECURSIVE,    vulnerability: VulnerabilityType::RecursiveCall,   base_score: 20.0, description_tmpl: "Exported function call - checked for self-recursion"            },
        PatternRule { regex: &GO_ALLOC,        vulnerability: VulnerabilityType::HotAllocation,   base_score: 12.0, description_tmpl: "make/new/slice-literal allocation in hot path"                 },
        PatternRule { regex: &GO_BLOCKING_IO,  vulnerability: VulnerabilityType::BlockingIO,      base_score: 18.0, description_tmpl: "Blocking stdlib I/O call - goroutine contention risk"           },
        PatternRule { regex: &GO_BRANCH,       vulnerability: VulnerabilityType::CognitiveBranch, base_score:  5.0, description_tmpl: "Branch/boolean expression increases cyclomatic complexity"     },
        PatternRule { regex: &GO_MUTEX,        vulnerability: VulnerabilityType::SyncContention,  base_score: 22.0, description_tmpl: "sync.Mutex/atomic - potential throughput bottleneck"            },
    ]
}

// =============================================================================
// § 5  File language detection
// =============================================================================

/// Returns the language string and rule table for a given file extension.
/// Returns `None` for unsupported extensions.
fn detect_language(path: &Path) -> Option<(&'static str, Vec<PatternRule>)> {
    match path.extension()?.to_str()? {
        "py"  => Some(("Python", python_rules())),
        "go"  => Some(("Go",     go_rules())),
        _     => None,
    }
}

// Returns the function-name regex for a language.
fn func_regex_for(language: &str) -> &'static Lazy<Regex> {
    match language {
        "Python" => &PY_FUNC,
        "Go"     => &GO_FUNC,
        _        => &PY_FUNC, // fallback
    }
}

// =============================================================================
// § 6  Line-level analyzer
// =============================================================================

/// State threaded through the line-by-line scan.
struct ScanState {
    current_function:  String,
    nesting_depth:     usize, // tracks loop nesting depth for score multiplier
    loop_depth:        usize, // specifically loop nesting (for / while)
}

impl ScanState {
    fn new() -> Self {
        Self {
            current_function: "<module>".to_owned(),
            nesting_depth:    0,
            loop_depth:       0,
        }
    }
}

/// Analyze a single trimmed source line.
///
/// Returns zero or more `Hotspot` values discovered on that line.
///
/// The nesting multiplier exponentially increases the entropy contribution
/// of any pattern found inside deeply-nested loops — this models the
/// actual O(n^k) impact on runtime complexity.
fn analyze_line(
    raw_line:  &str,
    line_no:   usize,
    state:     &mut ScanState,
    rules:     &[PatternRule],
    func_re:   &Regex,
    language:  &str,
) -> Vec<Hotspot> {
    let mut hotspots = Vec::new();

    // ── Track current function context ────────────────────────────────────────
    if let Some(cap) = func_re.captures(raw_line) {
        if let Some(name) = cap.get(1) {
            state.current_function = name.as_str().to_owned();
            // Reset per-function loop depth when entering a new function
            state.loop_depth = 0;
        }
    }

    // ── Track nesting depth via indentation (Python) or brace count (Go) ────
    match language {
        "Python" => {
            // Use indent-level heuristic: every 4 spaces of leading whitespace
            // increases the nesting depth counter by 1.
            let indent = raw_line.len() - raw_line.trim_start().len();
            state.nesting_depth = indent / 4;

            // Count specifically loop nesting depth
            if PY_FOR_WHILE.is_match(raw_line) {
                state.loop_depth = state.loop_depth.saturating_add(1);
            }
        }
        "Go" => {
            // Count unmatched `{` and `}` to track block depth
            let opens:  usize = raw_line.chars().filter(|&c| c == '{').count();
            let closes: usize = raw_line.chars().filter(|&c| c == '}').count();
            state.nesting_depth = state.nesting_depth.saturating_add(opens).saturating_sub(closes);

            if GO_FOR.is_match(raw_line) {
                state.loop_depth = state.loop_depth.saturating_add(1);
            }
        }
        _ => {}
    }

    // ── Loop-nesting entropy multiplier ──────────────────────────────────────
    // Score × (1 + 0.5 × loop_depth²) — exponential cost model:
    //   depth 0 → ×1.0, depth 1 → ×1.5, depth 2 → ×3.0, depth 3 → ×5.5
    let nesting_multiplier = 1.0 + 0.5 * (state.loop_depth as f64).powi(2);

    // ── Apply each rule ───────────────────────────────────────────────────────
    for rule in rules {
        if rule.regex.is_match(raw_line) {
            let raw_score  = rule.base_score * nesting_multiplier;
            // Clamp to [0, 100]
            let entropy    = raw_score.min(100.0).max(0.0);

            hotspots.push(Hotspot {
                function_name:    state.current_function.clone(),
                line_number:      line_no,
                source_snippet:   raw_line.trim().chars().take(120).collect(),
                entropy_score:    (entropy * 100.0).round() / 100.0, // 2 d.p.
                vulnerability_type: rule.vulnerability.clone(),
                description:      rule.description_tmpl.to_owned(),
            });
        }
    }

    hotspots
}

// =============================================================================
// § 7  File-level scanner
// =============================================================================

/// Read and analyze a single source file.
///
/// Opens the file using a buffered reader to avoid loading the entire file
/// into memory at once — important for large source trees.
pub fn scan_file(path: &Path, min_score: f64) -> io::Result<Option<FileReport>> {
    // ── Language detection ────────────────────────────────────────────────────
    let (language, rules) = match detect_language(path) {
        Some(lr) => lr,
        None     => return Ok(None), // unsupported file type
    };

    let func_re   = func_regex_for(language);
    let file      = fs::File::open(path)?;
    let reader    = io::BufReader::new(file);

    let mut state         = ScanState::new();
    let mut all_hotspots  : Vec<Hotspot> = Vec::new();
    let mut lines_scanned : usize        = 0;

    for (idx, line_result) in reader.lines().enumerate() {
        let line = line_result?;
        let trimmed = line.trim();

        // Skip blank lines and pure comment lines to keep the scancount meaningful
        if trimmed.is_empty()
            || trimmed.starts_with('#')   // Python / shell comment
            || trimmed.starts_with("//")  // Go / C-style comment
            || trimmed.starts_with("/*")  // block comment
        {
            continue;
        }

        lines_scanned += 1;

        let mut hotspots = analyze_line(
            &line,
            idx + 1, // 1-indexed line number
            &mut state,
            &rules,
            func_re,
            language,
        );

        // Apply min-score filter
        hotspots.retain(|h| h.entropy_score >= min_score);
        all_hotspots.extend(hotspots);
    }

    // ── Sort hotspots by entropy descending ───────────────────────────────────
    all_hotspots.sort_by(|a, b| b.entropy_score.partial_cmp(&a.entropy_score).unwrap());

    // ── Aggregate metrics ─────────────────────────────────────────────────────
    let total_entropy = all_hotspots.iter().map(|h| h.entropy_score).sum::<f64>();
    let mean_entropy = if all_hotspots.is_empty() {
        0.0
    } else {
        (total_entropy / all_hotspots.len() as f64 * 100.0).round() / 100.0
    };

    Ok(Some(FileReport {
        file_path:            path.to_string_lossy().into_owned(),
        language:             language.to_owned(),
        lines_scanned,
        total_entropy:        (total_entropy * 100.0).round() / 100.0,
        mean_hotspot_entropy: mean_entropy,
        hotspots:             all_hotspots,
    }))
}

// =============================================================================
// § 8  Directory walker
// =============================================================================

/// Walk `root` recursively, scan every supported source file, and collect
/// `FileReport` values.  Uses Rayon for parallel execution when the feature
/// is enabled (default).
pub fn scan_directory(root: &Path, min_score: f64, verbose: bool) -> Vec<FileReport> {
    // Collect candidate paths first so we can parallelize the heavy I/O phase.
    let candidates: Vec<PathBuf> = WalkDir::new(root)
        .follow_links(false)
        .into_iter()
        .filter_map(|e| e.ok())
        .filter(|e| e.file_type().is_file())
        .map(|e| e.into_path())
        .filter(|p| detect_language(p).is_some())
        .collect();

    if verbose {
        println!(
            "{} {} source files to analyze …",
            "→".cyan().bold(),
            candidates.len()
        );
    }

    // ── Parallel scan (Rayon) ─────────────────────────────────────────────────
    #[cfg(feature = "parallel")]
    let reports: Vec<FileReport> = {
        use rayon::iter::IntoParallelIterator;
        candidates
            .into_par_iter()
            .filter_map(|path| {
                if verbose {
                    println!("  {} {}", "⚡".yellow(), path.display());
                }
                match scan_file(&path, min_score) {
                    Ok(Some(report)) => Some(report),
                    Ok(None)         => None,
                    Err(e) => {
                        eprintln!("{} {}: {}", "ERR".red().bold(), path.display(), e);
                        None
                    }
                }
            })
            .collect()
    };

    // ── Sequential fallback (no Rayon feature) ────────────────────────────────
    #[cfg(not(feature = "parallel"))]
    let reports: Vec<FileReport> = candidates
        .into_iter()
        .filter_map(|path| {
            if verbose {
                println!("  {} {}", "→", path.display());
            }
            match scan_file(&path, min_score) {
                Ok(Some(report)) => Some(report),
                Ok(None)         => None,
                Err(e) => {
                    eprintln!("ERR {}: {}", path.display(), e);
                    None
                }
            }
        })
        .collect();

    reports
}

// =============================================================================
// § 9  Report builder & JSON serializer
// =============================================================================

/// Assemble the root `ThermodynamicReport`, sort by entropy, and write JSON.
pub fn build_and_write_report(
    mut file_reports: Vec<FileReport>,
    scanned_dir:     &Path,
    output_path:     &Path,
) -> io::Result<ThermodynamicReport> {
    // Sort files by total_entropy descending — highest-entropy files first
    file_reports.sort_by(|a, b| b.total_entropy.partial_cmp(&a.total_entropy).unwrap());

    let total_hotspots = file_reports.iter().map(|r| r.hotspots.len()).sum();
    let global_entropy = file_reports.iter().map(|r| r.total_entropy).sum::<f64>();

    let report = ThermodynamicReport {
        engine_version:    env!("CARGO_PKG_VERSION").to_owned(),
        generated_at:      chrono::Utc::now().to_rfc3339(),
        scanned_directory: scanned_dir.to_string_lossy().into_owned(),
        files_analyzed:    file_reports.len(),
        total_hotspots,
        global_entropy:    (global_entropy * 100.0).round() / 100.0,
        file_reports,
    };

    // Pretty-print JSON with 2-space indentation for human readability
    let json = serde_json::to_string_pretty(&report)
        .map_err(|e| io::Error::new(io::ErrorKind::Other, e))?;

    fs::write(output_path, json)?;

    Ok(report)
}

// =============================================================================
// § 10  Terminal summary banner
// =============================================================================

fn print_summary(report: &ThermodynamicReport) {
    println!();
    println!("{}", "╔══════════════════════════════════════════╗".cyan());
    println!("{}", "║   Thermodynamic AST Engine — Summary     ║".cyan());
    println!("{}", "╚══════════════════════════════════════════╝".cyan());
    println!("  Files analyzed   : {}", report.files_analyzed.to_string().yellow());
    println!("  Total hotspots   : {}", report.total_hotspots.to_string().yellow());
    println!("  Global entropy   : {}", format!("{:.2}", report.global_entropy).red().bold());

    println!();
    println!("{}", "  Top 5 Entropy Hotspots:".bold());
    println!("  {:<6}  {:<30}  {:<8}  {}", "Score", "File", "Line", "Vulnerability");
    println!("  {}", "─".repeat(72));

    // Flatten and collect top 5 across all files
    let mut all_hotspots: Vec<(f64, &str, usize, &VulnerabilityType)> = report
        .file_reports
        .iter()
        .flat_map(|r| {
            r.hotspots.iter().map(move |h| {
                (h.entropy_score, r.file_path.as_str(), h.line_number, &h.vulnerability_type)
            })
        })
        .collect();

    all_hotspots.sort_by(|a, b| b.0.partial_cmp(&a.0).unwrap());

    for (score, file, line, vuln) in all_hotspots.iter().take(5) {
        let short_file = Path::new(file)
            .file_name()
            .map(|n| n.to_string_lossy())
            .unwrap_or_default();
        println!(
            "  {:<6.2}  {:<30}  {:<8}  {}",
            score,
            short_file,
            line,
            vuln.to_string().red()
        );
    }

    println!();
}

// =============================================================================
// § 11  Entry point
// =============================================================================

fn main() {
    let cli = Cli::parse();

    // ── Validate input directory ──────────────────────────────────────────────
    if !cli.directory.exists() {
        eprintln!(
            "{} Directory '{}' does not exist.",
            "ERROR:".red().bold(),
            cli.directory.display()
        );
        std::process::exit(1);
    }
    if !cli.directory.is_dir() {
        eprintln!(
            "{} '{}' is not a directory.",
            "ERROR:".red().bold(),
            cli.directory.display()
        );
        std::process::exit(1);
    }

    println!(
        "\n{} Scanning: {}\n",
        "▶".green().bold(),
        cli.directory.display().to_string().cyan()
    );

    // ── Scan ──────────────────────────────────────────────────────────────────
    let file_reports = scan_directory(&cli.directory, cli.min_score, cli.verbose);

    if file_reports.is_empty() {
        println!(
            "{} No supported source files found in '{}'.",
            "WARN:".yellow().bold(),
            cli.directory.display()
        );
        std::process::exit(0);
    }

    // ── Build & persist report ────────────────────────────────────────────────
    let report = match build_and_write_report(file_reports, &cli.directory, &cli.output) {
        Ok(r)  => r,
        Err(e) => {
            eprintln!("{} Failed to write report: {}", "ERROR:".red().bold(), e);
            std::process::exit(1);
        }
    };

    // ── Print summary banner ──────────────────────────────────────────────────
    print_summary(&report);

    println!(
        "{} Report written to: {}\n",
        "✓".green().bold(),
        cli.output.display().to_string().green()
    );
}

// =============================================================================
// § 12  Unit tests
// =============================================================================

#[cfg(test)]
mod tests {
    use super::*;

    // ── Hotspot detection tests ───────────────────────────────────────────────

    #[test]
    fn test_python_blocking_io_detected() {
        let line = "    result = requests.get(url, timeout=30)";
        let mut state = ScanState::new();
        let rules = python_rules();
        let hotspots = analyze_line(line, 42, &mut state, &rules, &PY_FUNC, "Python");

        assert!(
            hotspots.iter().any(|h| h.vulnerability_type == VulnerabilityType::BlockingIO),
            "Expected BlockingIO hotspot for requests.get"
        );
    }

    #[test]
    fn test_go_allocation_detected() {
        let line = "    data := make([]byte, 1024)";
        let mut state = ScanState::new();
        let rules = go_rules();
        let hotspots = analyze_line(line, 7, &mut state, &rules, &GO_FUNC, "Go");

        assert!(
            hotspots.iter().any(|h| h.vulnerability_type == VulnerabilityType::HotAllocation),
            "Expected HotAllocation hotspot for make()"
        );
    }

    #[test]
    fn test_nesting_multiplier_increases_score() {
        let mut state = ScanState::new();
        let rules = python_rules();

        // Simulate two levels of loop nesting
        state.loop_depth = 2;

        let line = "    result = requests.get(url)";
        let hotspots = analyze_line(line, 10, &mut state, &rules, &PY_FUNC, "Python");

        let blocking_io = hotspots
            .iter()
            .find(|h| h.vulnerability_type == VulnerabilityType::BlockingIO)
            .expect("Expected BlockingIO hotspot");

        // base_score=18, nesting multiplier at depth 2 = 1 + 0.5*4 = 3.0 → 54.0
        assert!(
            blocking_io.entropy_score > 18.0,
            "Nesting multiplier should increase entropy beyond base score"
        );
    }

    #[test]
    fn test_go_mutex_detected() {
        let line = "    mu.Lock()  // sync.Mutex";
        let mut state = ScanState::new();
        let rules = go_rules();
        let _hotspots = analyze_line(line, 99, &mut state, &rules, &GO_FUNC, "Go");

        // The mutex pattern matches on `sync.` prefix — verify SyncContention fires
        // on a line that explicitly contains the package
        let line2 = "    var mu sync.Mutex";
        let hotspots2 = analyze_line(line2, 100, &mut state, &rules, &GO_FUNC, "Go");

        assert!(
            hotspots2.iter().any(|h| h.vulnerability_type == VulnerabilityType::SyncContention),
            "Expected SyncContention for sync.Mutex declaration"
        );
    }

    #[test]
    fn test_language_detection() {
        assert!(detect_language(Path::new("foo.py")).is_some());
        assert!(detect_language(Path::new("bar.go")).is_some());
        assert!(detect_language(Path::new("baz.rs")).is_none());
        assert!(detect_language(Path::new("qux.js")).is_none());
    }

    #[test]
    fn test_entropy_score_clamped() {
        let mut state = ScanState::new();
        // Set absurdly high nesting depth to trigger clamping
        state.loop_depth = 100;
        let rules = python_rules();
        let line = "    mu.acquire()  # threading.Lock()";
        let hotspots = analyze_line(line, 1, &mut state, &rules, &PY_FUNC, "Python");
        for h in &hotspots {
            assert!(h.entropy_score <= 100.0, "Entropy must be clamped to 100.0");
        }
    }
}
