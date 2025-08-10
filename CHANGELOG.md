# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- TBD

### Changed
- TBD

### Fixed
- TBD

### Security
- TBD

## [v0.1.0-beta.1] - 2025-08-10

### Added
- Initial CLI implementation with `analyze` and `codegen` commands
- Support for Go, Python, JavaScript, Java, .NET, Ruby, PHP detection
- OTEL instrumentation analysis and recommendations
- Template-based code generation with `--dry-run`
- GoReleaser config and GitHub Actions workflow for releases
- Homebrew tap publishing via GoReleaser
- Install script for prebuilt archives

### Changed
- Updated README to reflect implemented commands and installation paths

### Fixed
- Improved Homebrew formula test to assert `--version`
