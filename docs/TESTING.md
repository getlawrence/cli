# Testing Guide for Lawrence CLI

This document describes the testing setup and how to run tests for the Lawrence CLI project.

## Test Structure

The tests are organized following Go best practices:

```
internal/
├── detector/
│   ├── language_detector_test.go  # Core language detection tests
│   └── languages/
│       ├── go_test.go             # Go language detector tests
│       └── python_test.go         # Python language detector tests
└── testdata/                      # Test data files
    └── sample_projects/           # Sample projects for integration tests
        ├── go_otel_project/
        └── python_otel_project/
```

## Running Tests

### Using Make Commands

The project includes several Make targets for testing:

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run tests with coverage and generate HTML report
make test-coverage-html

# Run only detector tests
make test-detector

# Run tests in watch mode (requires entr)
make test-watch
```

### Using Go Commands Directly

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests with coverage
go test -cover ./...

# Run only detector tests
go test ./internal/detector/...

# Run specific test function
go test -run TestDetectLanguages ./internal/detector/

# Run tests with race detection
go test -race ./...
```

### Using the Test Runner Script

```bash
# Run all tests with the custom test runner
./run_tests.sh

# Run specific package tests
./run_tests.sh ./internal/detector/
```

## Test Categories

### Unit Tests

**Language Detector Tests (`language_detector_test.go`)**:
- `TestDetectLanguages`: Tests language detection across directory structures
- `TestDetectLanguageForFile`: Tests single file language detection
- `TestShouldSkipFile`: Tests file/directory filtering logic
- `TestIsProgrammingLanguage`: Tests programming language classification
- `TestNormalizeDirectoryKey`: Tests directory path normalization
- `TestFindMostCommonLanguage`: Tests language selection logic

**Go Detector Tests (`go_test.go`)**:
- `TestGoDetectorName`: Tests detector name
- `TestGoDetectorDetect`: Tests Go project detection
- `TestGoDetectorGetOTelLibraries`: Tests OpenTelemetry library detection
- `TestGoDetectorGetAllPackages`: Tests package detection
- `TestGoDetectorGetFilePatterns`: Tests file pattern matching
- `TestGoDetectorIsThirdPartyPackage`: Tests third-party package identification
- `TestGoDetectorFindGoFiles`: Tests Go file discovery

**Python Detector Tests (`python_test.go`)**:
- `TestPythonDetectorName`: Tests detector name
- `TestPythonDetectorDetect`: Tests Python project detection
- `TestPythonDetectorGetFilePatterns`: Tests file pattern matching
- `TestPythonDetectorIsThirdPartyPackage`: Tests third-party package identification

### Test Data

The `testdata/` directory contains sample projects for integration testing:

- **go_otel_project/**: Go project with OpenTelemetry dependencies
- **python_otel_project/**: Python project with OpenTelemetry dependencies

## Test Best Practices

1. **Isolation**: Each test creates its own temporary directory to avoid conflicts
2. **Cleanup**: Tests use `defer os.RemoveAll(tempDir)` for proper cleanup
3. **Table-driven**: Tests use table-driven patterns for multiple scenarios
4. **Descriptive naming**: Test functions and subtests have clear, descriptive names
5. **Error handling**: Tests properly handle and report errors
6. **No external dependencies**: Tests don't require external services or internet access

## Coverage

To view test coverage:

```bash
# Generate coverage report
make test-coverage-html

# Open coverage report in browser
open coverage.html
```

## Continuous Integration

Tests are designed to run in CI environments:

- No external dependencies required
- All tests use temporary directories
- Tests are deterministic and don't rely on system state
- Race condition detection enabled

## Adding New Tests

When adding new tests:

1. Follow the existing naming conventions (`Test<Function>Name`)
2. Use table-driven tests for multiple scenarios
3. Create temporary directories for file-based tests
4. Add appropriate cleanup with `defer`
5. Include both positive and negative test cases
6. Test error conditions appropriately

## Known Limitations

1. The Go detector's `Detect` method has a limitation with `filepath.Glob` not supporting `**/*.go` patterns. This is documented in the test as expected behavior.
2. Some lint warnings about code complexity are expected due to comprehensive test coverage.

## Troubleshooting

If tests fail:

1. Check that you're running from the project root directory
2. Ensure Go modules are up to date: `go mod tidy`
3. Run tests with verbose output: `go test -v ./...`
4. Check for race conditions: `go test -race ./...`
5. Review test output for specific failure details
