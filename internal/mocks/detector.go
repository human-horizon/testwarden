// Package mocks detects over-mocking patterns in unit tests.
package mocks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Violation represents a single over-mocking finding.
type Violation struct {
	File    string
	Line    int
	RealDep string
	MockLib string
}

// Detector scans files for over-mocking patterns.
type Detector struct {
	realDeps map[string][]string
}

// New creates a Detector with the configured real dependencies.
func New(realDeps map[string][]string) *Detector {
	return &Detector{realDeps: realDeps}
}

// ScanDir walks a directory and returns over-mocking violations.
// language is "go" or "typescript".
func (d *Detector) ScanDir(root, language string) ([]Violation, error) {
	switch language {
	case "go":
		return d.scanGoDir(root)
	case "typescript":
		return d.scanTSDir(root)
	}
	return nil, nil
}

// DetectGoFile scans a single Go test file for over-mocking.
func (d *Detector) DetectGoFile(path string) []Violation {
	realSet := make(map[string]bool)
	for _, dep := range d.realDeps["go"] {
		realSet[dep] = true
	}
	return d.scanGoFile(path, realSet)
}

// DetectTSFile scans a single TypeScript test file for over-mocking.
func (d *Detector) DetectTSFile(path string) []Violation {
	realSet := d.realDeps["typescript"]

	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	content := string(data)

	hasMockCall := strings.Contains(content, "jest.mock(") ||
		strings.Contains(content, "sinon.mock(") ||
		strings.Contains(content, ".mock(")

	var matchedDep string
	for _, real := range realSet {
		if strings.Contains(content, real) {
			matchedDep = real
			break
		}
	}

	if hasMockCall && matchedDep != "" {
		return []Violation{{
			File:    path,
			RealDep: matchedDep,
			MockLib: "jest/sinon",
		}}
	}
	return nil
}

func (d *Detector) scanGoDir(root string) ([]Violation, error) {
	var violations []Violation
	realSet := make(map[string]bool)
	for _, dep := range d.realDeps["go"] {
		realSet[dep] = true
	}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			name := info.Name()
			if name == "vendor" || name == ".git" || strings.HasPrefix(name, ".") {
				if path != root {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		violations = append(violations, d.scanGoFile(path, realSet)...)
		return nil
	})

	return violations, err
}

// scanGoFile uses real Go AST parsing to detect over-mocking.
func (d *Detector) scanGoFile(path string, realSet map[string]bool) []Violation {
	var violations []Violation

	parsed, err := parseGoAST(path)
	if err != nil {
		return violations
	}

	var firstMockLib string
	var firstRealDep string

	for _, imp := range parsed.imports {
		if mockLib := mockLibForImport(imp); mockLib != "" && firstMockLib == "" {
			firstMockLib = mockLib
		}
		if realDep := realDepForImport(imp, realSet); realDep != "" && firstRealDep == "" {
			firstRealDep = realDep
		}
	}

	if firstMockLib != "" && firstRealDep != "" {
		line := 0
		if len(parsed.mockAssigns) > 0 {
			line = parsed.mockAssigns[0]
		}
		violations = append(violations, Violation{
			File:    path,
			Line:    line,
			RealDep: firstRealDep,
			MockLib: firstMockLib,
		})
	}

	return violations
}

func (d *Detector) scanTSDir(root string) ([]Violation, error) {
	var violations []Violation
	realSet := d.realDeps["typescript"]

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			name := info.Name()
			if name == "node_modules" || name == "dist" || name == ".git" || strings.HasPrefix(name, ".") {
				if path != root {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, ".ts") && !strings.HasSuffix(path, ".tsx") {
			return nil
		}
		isTest := strings.Contains(path, ".test.") || strings.Contains(path, ".spec.") ||
			strings.HasSuffix(path, "Test.ts") || strings.HasSuffix(path, "Spec.ts")
		if !isTest {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		data, err := readAll(f)
		if err != nil {
			return nil
		}
		content := string(data)

		hasMockCall := strings.Contains(content, "jest.mock(") ||
			strings.Contains(content, "sinon.mock(") ||
			strings.Contains(content, ".mock(")
		var matchedDep string
		for _, real := range realSet {
			if strings.Contains(content, real) {
				matchedDep = real
				break
			}
		}

		if hasMockCall && matchedDep != "" {
			violations = append(violations, Violation{
				File:    path,
				RealDep: matchedDep,
				MockLib: "jest/sinon",
			})
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk: %w", err)
	}
	return violations, nil
}

func readAll(f *os.File) ([]byte, error) {
	var buf [4094]byte
	var out []byte
	for {
		n, err := f.Read(buf[:])
		out = append(out, buf[:n]...)
		if err != nil {
			if err.Error() == "EOF" {
				return out, nil
			}
			return out, err
		}
		if n == 0 {
			return out, nil
		}
	}
}
