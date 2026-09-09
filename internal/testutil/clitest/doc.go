// Package clitest builds the pobear binary (call Build once and reuse the
// path across fixtures) and, per fixture, runs it as a subprocess and diffs
// stdout/stderr/exit code against a single YAML case file (args, stdout,
// stderr, exit, and optional stdin/providers/fixtures keys) — the exact
// output/exit-code behavior each fixture predicts comes from
// docs/CLI_REFERENCE.md.
package clitest
