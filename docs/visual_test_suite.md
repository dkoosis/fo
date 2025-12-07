# Visual Test Suite for fo Rendering

## Overview

The visual test suite provides a comprehensive set of test scenarios for all of fo's rendering capabilities. This allows for systematic review and iteration on the visual design, ensuring consistency and quality across all output types.

## Quick Start

Run the visual test suite:

```bash
make visual-test
```

Or directly:

```bash
go run cmd/visual_test_main.go visual_test_outputs/
```

## Test Scenarios

The suite generates 12 different test scenarios, each saved to a separate file:

### 1. Section Headers (`01_section_headers.txt`)
Tests basic section header rendering with multiple sections.

### 2. Test Results - All Pass (`02_test_results_all_pass.txt`)
Shows test results when all tests pass, with proper grouping and coverage visualization.

### 3. Test Results - With Failures (`03_test_results_with_failures.txt`)
Demonstrates test output with failed tests, including failed test name display.

### 4. Test Results - Mixed Coverage (`04_test_results_mixed_coverage.txt`)
Shows test results with varying coverage levels (good, warning, error thresholds).

### 5. Quality Checks - All Pass (`05_quality_checks_all_pass.txt`)
Quality check sections when all checks pass.

### 6. Quality Checks - With Warnings (`06_quality_checks_with_warnings.txt`)
Quality checks with warning-level issues (non-fatal).

### 7. Build Workflow (`07_build_workflow.txt`)
Complete build workflow showing sections with nested test output.

### 8. Sections with Nested Content (`08_sections_nested_content.txt`)
Sections containing indented/nested content structures.

### 9. Live Sections (`09_live_sections.txt`)
Live updating sections that change during execution.

### 10. Error Scenarios (`10_error_scenarios.txt`)
Error and warning scenarios to test error display.

### 11. Long Content (`11_long_content.txt`)
Long content that needs proper wrapping and alignment.

### 12. Multiple Themes (`12_multiple_themes.txt`)
Placeholder for theme testing (can be extended).

## Use Cases

### Design Review
1. Run the suite: `make visual-test`
2. Review all generated files
3. Identify visual inconsistencies or issues
4. Make improvements to fo's rendering code
5. Re-run to see changes

### Visual Regression Testing
1. Run suite before changes: `make visual-test`
2. Save outputs as baseline
3. Make code changes
4. Run suite again and compare outputs
5. Verify improvements or catch regressions

### Design Iteration
1. Focus on specific scenarios (e.g., test results)
2. Make targeted improvements
3. Re-run to see impact
4. Iterate until satisfied

## Integration

The visual test suite can be integrated into:

- **Development workflow**: Run before committing changes
- **CI/CD pipeline**: Generate outputs for comparison
- **Documentation**: Use outputs as examples
- **Design reviews**: Share outputs for feedback

## Extending the Suite

To add new test scenarios:

1. Create a new test function:
   ```go
   func testNewScenario(console *fo.Console, buf *bytes.Buffer) error {
       // Your test code
       return nil
   }
   ```

2. Add to the scenarios slice in `main()`:
   ```go
   {
       name:     "New Scenario",
       run:      testNewScenario,
       filename: "13_new_scenario.txt",
   },
   ```

## Output Format

All outputs are saved as text files with ANSI color codes preserved. This allows:
- Review in terminal with colors
- Review in text editors
- Comparison using diff tools
- Version control (if desired)

## Related Documentation

- [Theme Guide](../guides/THEME_GUIDE.md) - Customizing themes
- [Architecture](../design/architecture.md) - Understanding fo's architecture
- [Patterns](../design/pattern-types.md) - Available rendering patterns

