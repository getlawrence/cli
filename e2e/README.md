# End-to-End Testing Guide

This document describes the comprehensive end-to-end testing framework for the Lawrence CLI.

## Overview

The E2E testing framework validates the Lawrence CLI behavior in clean Docker containers using the example projects. Tests verify:

- Language detection accuracy
- Package and dependency detection
- OpenTelemetry library identification
- Instrumentation opportunity detection
- Output format correctness
- Error handling

## Test Structure

```
e2e/
├── run-tests.sh              # Main test runner
├── test-utils.sh             # Utility functions and validation
├── test-instrumentation.sh   # Instrumentation-specific tests
└── results/                  # Test output and reports (created during runs)

examples/
├── go-sample/                # Go project without OTEL
└── python-sample/           # Python project without OTEL
```

## Running E2E Tests

### Quick Start

```bash
# Run all E2E tests
make e2e-test

# Run instrumentation-specific tests
make e2e-test-instrumentation

# Run tests on multiple distributions (CI only)
make e2e-test-multi

# Clean up test artifacts
make e2e-clean
```

### Docker Compose

```bash
# Run basic E2E test suite
docker-compose -f docker-compose.e2e.yml up --build lawrence-e2e

# Run instrumentation detection tests
docker-compose -f docker-compose.e2e.yml up --build lawrence-instrumentation-tests

# Interactive shell for debugging
docker-compose -f docker-compose.e2e.yml run --rm lawrence-e2e /bin/bash
```

### Direct Docker

```bash
# Build and run E2E tests
docker build -f Dockerfile.e2e -t lawrence-e2e .
docker run --rm lawrence-e2e

# Run with volume for results
docker run --rm -v $(pwd)/e2e-results:/test/results lawrence-e2e
```

## Test Scenarios

### 1. Basic CLI Functionality
- Version command works
- Help command displays correctly
- Language listing is accurate
- Error handling for invalid inputs

### 2. Language Detection
- **Go Projects**: Detects via `go.mod` and `.go` files
- **Python Projects**: Detects via `requirements.txt`, `pyproject.toml`, `setup.py`
- **Mixed Projects**: Correctly identifies multiple languages
- **Empty Projects**: Handles gracefully

### 3. Package Detection

#### Go Sample Project Expected Results:
```json
{
  "languages": {"root": "go"},
  "packages": [
    {"name": "github.com/gin-gonic/gin", "language": "go"},
    {"name": "github.com/gorilla/mux", "language": "go"},
    {"name": "github.com/go-redis/redis/v8", "language": "go"}
  ]
}
```

#### Python Sample Project Expected Results:
```json
{
  "languages": {"root": "python"},
  "packages": [
    {"name": "flask", "language": "python"},
    {"name": "requests", "language": "python"},
    {"name": "psycopg2", "language": "python"},
    {"name": "redis", "language": "python"},
    {"name": "sqlalchemy", "language": "python"}
  ]
}
```

### 4. Instrumentation Detection

#### Non-Instrumented Projects:
- Should NOT detect OpenTelemetry libraries
- Should suggest instrumentation opportunities
- Should identify frameworks that can be instrumented

#### Instrumented Projects:
- Should detect OpenTelemetry libraries correctly
- Should identify which components are already instrumented
- Should provide fewer/different recommendations

### 5. Output Format Validation
- **JSON**: Valid JSON structure with expected fields
- **Default**: Human-readable format works
- **Error Cases**: Proper error messages and exit codes

## Test Implementation Details

### Test Utilities (`test-utils.sh`)

**Validation Functions:**
- `validate_package_detection()` - Verifies expected packages are found
- `validate_language_detection()` - Confirms correct language identification
- `validate_instrumentation_opportunities()` - Checks instrumentation suggestions

**Test Data Generators:**
- `generate_test_go_project()` - Creates instrumented Go project
- `generate_test_python_project()` - Creates instrumented Python project

**Performance Testing:**
- `run_performance_tests()` - Ensures analysis completes in reasonable time

### Assertion Functions

```bash
# Command success/failure
assert_command_succeeds "Description" command args
assert_command_fails "Description" command args

# Output validation
assert_output_contains "Description" "expected_text" command args
assert_json_contains "Description" ".json.path" "expected_value" command args
```

### Expected Test Behavior

1. **Go Sample Analysis**:
   ```bash
   lawrence analyze --format json
   # Should detect: gin, gorilla/mux, redis packages
   # Should suggest: HTTP instrumentation
   # Should NOT show: OpenTelemetry libraries
   ```

2. **Python Sample Analysis**:
   ```bash
   lawrence analyze --format json
   # Should detect: flask, requests, psycopg2, redis, sqlalchemy
   # Should suggest: Flask instrumentation
   # Should NOT show: OpenTelemetry libraries
   ```

3. **Mixed Project**:
   ```bash
   lawrence analyze --format json
   # Should detect: Both Go and Python in respective directories
   # Should provide: Language-specific recommendations
   ```

## CI/CD Integration

### GitHub Actions Workflow

The E2E tests run automatically on:
- Push to main/develop branches
- Pull requests
- Manual workflow dispatch

**Test Matrix:**
- Basic functionality tests
- Instrumentation detection tests
- Multi-distribution tests (main branch only)

**Artifacts:**
- Test results and logs
- Coverage reports
- Performance metrics

## Debugging Failed Tests

### Interactive Debugging

```bash
# Start interactive shell in test environment
make e2e-shell

# Run specific test manually
/test/e2e/run-tests.sh

# Check CLI version and basic functionality
lawrence version
lawrence help
```

### Common Issues

1. **Language Detection Failures**:
   - Check if examples have expected file structure
   - Verify CLI build includes all dependencies

2. **Package Detection Issues**:
   - Ensure example projects have correct dependency files
   - Check if CLI properly parses package files

3. **JSON Output Problems**:
   - Validate JSON structure with `jq`
   - Check for proper error handling

4. **Performance Issues**:
   - Monitor test execution time
   - Check for resource constraints in container

### Viewing Test Results

```bash
# Check test output
cat e2e-results/test-output.log

# View detailed JSON responses
jq . e2e-results/analysis-results.json

# Check performance metrics
cat e2e-results/performance.log
```

## Extending E2E Tests

### Adding New Test Cases

1. **Create test function** in appropriate script:
   ```bash
   test_new_functionality() {
       log_info "=== Testing New Functionality ==="
       
       assert_command_succeeds "Description" lawrence new-command
       assert_output_contains "Expected output" "text" lawrence new-command
   }
   ```

2. **Add to main test runner**:
   ```bash
   # In run-tests.sh main() function
   test_new_functionality
   ```

3. **Update expected results** in test-utils.sh

### Adding New Languages

1. **Create example project** in `examples/new-language-sample/`
2. **Add expected results** to validation functions
3. **Update test scenarios** to include new language
4. **Add language-specific patterns** to detection tests

## Performance Benchmarks

Expected performance targets:
- Analysis of small projects: < 2 seconds
- Analysis of medium projects: < 5 seconds
- JSON output generation: < 1 second
- Memory usage: < 100MB

## Maintenance

### Regular Maintenance Tasks

1. **Update example projects** with latest dependencies
2. **Refresh expected test results** when CLI behavior changes
3. **Add tests for new features** and bug fixes
4. **Monitor test execution time** and optimize as needed

### Updating Dependencies

```bash
# Update Go example dependencies
cd examples/go-sample && go get -u && go mod tidy

# Update Python example dependencies
cd examples/python-sample && pip-compile requirements.in
```

This E2E testing framework ensures the Lawrence CLI works correctly across different environments and provides reliable analysis of real-world projects.
