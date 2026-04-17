# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2026-04-17

### Added
- Comprehensive testdata corpus in `pkg/transpiler/testdata/` for integration testing.
- GitHub Actions CI pipeline with 100% coverage enforcement for the transpiler core.
- MIT License and basic release plumbing.
- Performance benchmarks for `persistent.Map` and `persistent.List`.

### Changed
- **Breaking Documentation Change:** Reconciled pointer rules. Pointers are now permitted for type signatures and expressions (`*p`, `&x`) but remain read-only.
- Bumped Language Specification to v0.2.

### Fixed
- Stabilized documentation across all files (`README.md`, `CLAUDE.md`, `GEMINI.md`, etc.) to match `SPEC.md`.

## [0.1.0] - 2026-04-01

### Added
- Initial ImGo transpiler with SSA mangling and basic persistent collection support.
- Support for `map[K]V` and `[]T` using `pkg/persistent`.
- Deep updates with `SetIn`, `UpdateIn`, and `DeleteIn`.
