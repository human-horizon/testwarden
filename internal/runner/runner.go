// Package runner orchestrates check/fix across languages.
package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/HumanHorizon/testwarden/internal/ai"
	"github.com/HumanHorizon/testwarden/internal/cache"
	"github.com/HumanHorizon/testwarden/internal/config"
	"github.com/HumanHorizon/testwarden/internal/coverage"
	"github.com/HumanHorizon/testwarden/internal/mocks"
	"github.com/HumanHorizon/testwarden/internal/patcher"
	"github.com/HumanHorizon/testwarden/internal/report"
)

// Options control runner behavior.
type Options struct {
	Cfg    *config.Config
	Root   string
	JSON   bool
	DryRun bool
	NoTUI  bool
	Out    io.Writer
}

// RunCheck analyses the project and writes a report. Returns exit code (0/1).
func RunCheck(ctx context.Context, opts Options) (int, error) {
	progress := newProgress(opts)
	progress.AnalyseStarted()
	defer progress.Wait()

	results, err := analyze(ctx, opts)
	if err != nil {
		progress.Error(err)
		return 1, err
	}

	if opts.JSON {
		if err := report.PrintJSON(opts.Out, results); err != nil {
			return 1, err
		}
	} else {
		report.PrintText(opts.Out, results)
	}

	progress.Done(results)
	for _, r := range results {
		if !r.Passed {
			return 1, nil
		}
	}
	return 0, nil
}

// RunFix analyses the project and applies AI fixes, rolling back on test failure.
func RunFix(ctx context.Context, opts Options) (int, error) {
	progress := newProgress(opts)
	progress.AnalyseStarted()
	defer progress.Wait()

	results, err := analyze(ctx, opts)
	if err != nil {
		progress.Error(err)
		return 1, err
	}

	if opts.JSON {
		_ = report.PrintJSON(opts.Out, results)
	} else {
		report.PrintText(opts.Out, results)
	}

	aiClient := ai.New(ai.Config{
		Endpoint:  opts.Cfg.AI.Endpoint,
		APIKey:    opts.Cfg.AI.APIKey,
		Model:     opts.Cfg.AI.Model,
		Timeout:   opts.Cfg.AI.Timeout,
		MaxTokens: opts.Cfg.AI.MaxTokens,
	})
	p := patcher.New(opts.Root)

	type pendingIssue struct {
		result *report.Result
		issue  report.Issue
	}
	var pending []pendingIssue
	for _, r := range results {
		for _, issue := range r.Issues {
			if issue.File == "" {
				continue
			}
			pending = append(pending, pendingIssue{result: r, issue: issue})
		}
	}

	progress.FixStarted(len(pending))

	fixed := 0
	for i, item := range pending {
		progress.IssueStarted(i, len(pending), item.issue.Type, item.issue.File)
		if err := fixIssue(ctx, aiClient, p, progress, opts, item.result.Language, item.issue); err != nil {
			fmt.Fprintf(opts.Out, "  ✗ fix failed for %s: %v\n", item.issue.File, err)
			continue
		}
		fixed++
	}

	fmt.Fprintf(opts.Out, "\nFixed: %d issue(s)\n", fixed)
	progress.Done(results)
	if fixed > 0 {
		return 0, nil
	}
	for _, r := range results {
		if !r.Passed {
			return 1, nil
		}
	}
	return 0, nil
}

func analyze(ctx context.Context, opts Options) ([]*report.Result, error) {
	results := make([]*report.Result, len(opts.Cfg.Languages))
	g, ctx := errgroup.WithContext(ctx)

	for i, lang := range opts.Cfg.Languages {
		i, lang := i, lang
		g.Go(func() error {
			switch lang {
			case "go":
				r, err := analyzeGo(ctx, opts)
				if err != nil {
					return err
				}
				results[i] = r
			case "typescript":
				r, err := analyzeTS(ctx, opts)
				if err != nil {
					return err
				}
				results[i] = r
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	out := make([]*report.Result, 0, len(results))
	for _, r := range results {
		if r != nil {
			out = append(out, r)
		}
	}
	return out, nil
}

func analyzeGo(ctx context.Context, opts Options) (*report.Result, error) {
	result := &report.Result{Language: "go", Threshold: opts.Cfg.Coverage.UnitThreshold}

	if opts.Cfg.Mocks.DetectOvermocking {
		violations, err := detectGoMockViolations(opts.Root, opts.Cfg)
		if err != nil {
			return nil, fmt.Errorf("scan mocks: %w", err)
		}
		for _, v := range violations {
			result.Issues = append(result.Issues, report.Issue{
				Type:     "mock",
				Severity: "error",
				Message:  fmt.Sprintf("over-mocking: real dep %q with mock lib %q", v.RealDep, v.MockLib),
				File:     v.File,
				Line:     v.Line,
				Detail:   v.RealDep,
			})
		}
	}

	unitCovPath := filepath.Join(opts.Root, opts.Cfg.Coverage.UnitPath)
	if opts.Cfg.Coverage.UnitCommand != "" {
		if err := runShell(opts.Root, opts.Cfg.Coverage.UnitCommand); err != nil {
			fmt.Fprintf(opts.Out, "  (unit tests exited non-zero: %v)\n", err)
		}
	}
	unitRep, _ := coverage.ParseGo(unitCovPath)
	if unitRep != nil {
		result.Coverage = unitRep.Percent
	}

	var integrationRep *coverage.GoReport
	if opts.Cfg.Coverage.IntegrationPath != "" {
		integrationCovPath := filepath.Join(opts.Root, opts.Cfg.Coverage.IntegrationPath)
		if opts.Cfg.Coverage.IntegrationCommand != "" {
			if err := runShell(opts.Root, opts.Cfg.Coverage.IntegrationCommand); err != nil {
				fmt.Fprintf(opts.Out, "  (integration tests exited non-zero: %v)\n", err)
			}
		}
		integrationRep, _ = coverage.ParseGo(integrationCovPath)
	}

	gap := coverage.ComputeGap(unitRep, integrationRep)
	if gap.GapPercent > float64(opts.Cfg.Coverage.IntegrationGapThreshold) {
		result.Issues = append(result.Issues, report.Issue{
			Type:     "gap",
			Severity: "error",
			Message: fmt.Sprintf(
				"coverage gap %.2f%% exceeds threshold %d%%",
				gap.GapPercent, opts.Cfg.Coverage.IntegrationGapThreshold,
			),
			Value: gap.GapPercent,
		})
		for filePath, lines := range gap.ByFile {
			result.Issues = append(result.Issues, report.Issue{
				Type:     "gap",
				Severity: "error",
				Message:  fmt.Sprintf("lines uncovered by both unit and integration tests: %v", lines),
				File:     filePath,
			})
		}
	}

	if result.Coverage < float64(result.Threshold) {
		result.Issues = append(result.Issues, report.Issue{
			Type:     "coverage",
			Severity: "error",
			Message:  fmt.Sprintf("coverage %.2f%% below threshold %d%%", result.Coverage, result.Threshold),
			Value:    result.Coverage,
		})
	}

	result.Passed = len(result.Issues) == 0
	return result, nil
}

// detectGoMockViolations uses the cache to skip AST work for unchanged files.
func detectGoMockViolations(root string, cfg *config.Config) ([]mocks.Violation, error) {
	if !cfg.Mocks.DetectOvermocking {
		return nil, nil
	}

	c, err := cache.New(root)
	if err != nil {
		return nil, err
	}
	c.UseSuffix("go")
	manifest, err := c.LoadManifest()
	if err != nil {
		return nil, err
	}

	var allViolations []mocks.Violation
	files, err := collectGoTestFiles(root)
	if err != nil {
		return nil, err
	}

	d := mocks.New(cfg.Mocks.RealDependencies)

	for _, filePath := range files {
		hash, err := cache.HashFile(filePath)
		if err != nil {
			continue
		}
		if entry, _ := c.Lookup(filePath, hash); entry != nil {
			for _, v := range entry.Violations {
				allViolations = append(allViolations, mocks.Violation{
					File:    filePath,
					Line:    v.Line,
					RealDep: v.RealDep,
					MockLib: v.MockLib,
				})
			}
			continue
		}

		fileViolations := d.DetectGoFile(filePath)
		for _, v := range fileViolations {
			allViolations = append(allViolations, v)
		}

		entry := cache.Entry{Path: filePath, Hash: hash}
		for _, v := range fileViolations {
			entry.Violations = append(entry.Violations, cache.MockViolation{
				Line:    v.Line,
				RealDep: v.RealDep,
				MockLib: v.MockLib,
			})
		}
		manifest = c.Store(manifest, entry)
	}

	_ = c.Save(manifest)
	return allViolations, nil
}

// collectGoTestFiles walks root and returns paths ending in _test.go.
func collectGoTestFiles(root string) ([]string, error) {
	var out []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			name := info.Name()
			if name == "vendor" || name == ".git" || name == "node_modules" {
				if path != root {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if filepath.Ext(path) == ".go" && len(path) > 8 && path[len(path)-8:] == "_test.go" {
			out = append(out, path)
		}
		return nil
	})
	return out, err
}

// detectTSMockViolations uses the cache to skip per-file work for unchanged files.
func detectTSMockViolations(root string, cfg *config.Config) ([]mocks.Violation, error) {
	if !cfg.Mocks.DetectOvermocking {
		return nil, nil
	}

	c, err := cache.New(root)
	if err != nil {
		return nil, err
	}
	c.UseSuffix("ts")
	manifest, err := c.LoadManifest()
	if err != nil {
		return nil, err
	}

	var allViolations []mocks.Violation
	files, err := collectTSTestFiles(root)
	if err != nil {
		return nil, err
	}

	d := mocks.New(cfg.Mocks.RealDependencies)

	for _, filePath := range files {
		hash, err := cache.HashFile(filePath)
		if err != nil {
			continue
		}
		if entry, _ := c.Lookup(filePath, hash); entry != nil {
			for _, v := range entry.Violations {
				allViolations = append(allViolations, mocks.Violation{
					File:    filePath,
					Line:    v.Line,
					RealDep: v.RealDep,
					MockLib: v.MockLib,
				})
			}
			continue
		}

		fileViolations := d.DetectTSFile(filePath)
		for _, v := range fileViolations {
			allViolations = append(allViolations, v)
		}

		entry := cache.Entry{Path: filePath, Hash: hash}
		for _, v := range fileViolations {
			entry.Violations = append(entry.Violations, cache.MockViolation{
				Line:    v.Line,
				RealDep: v.RealDep,
				MockLib: v.MockLib,
			})
		}
		manifest = c.Store(manifest, entry)
	}

	_ = c.Save(manifest)
	return allViolations, nil
}

// collectTSTestFiles walks root and returns paths ending in .test.ts/.spec.ts/etc.
func collectTSTestFiles(root string) ([]string, error) {
	var out []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			name := info.Name()
			if name == "node_modules" || name == "dist" || name == ".git" || name == "vendor" || strings.HasPrefix(name, ".") {
				if path != root {
					return filepath.SkipDir
				}
			}
			return nil
		}
		base := filepath.Base(path)
		if strings.HasSuffix(base, ".test.ts") || strings.HasSuffix(base, ".spec.ts") ||
			strings.HasSuffix(base, ".test.tsx") || strings.HasSuffix(base, ".spec.tsx") ||
			strings.HasSuffix(base, "Test.ts") || strings.HasSuffix(base, "Spec.ts") {
			out = append(out, path)
		}
		return nil
	})
	return out, err
}

func analyzeTS(ctx context.Context, opts Options) (*report.Result, error) {
	result := &report.Result{Language: "typescript", Threshold: opts.Cfg.Coverage.UnitThreshold}

	if opts.Cfg.Mocks.DetectOvermocking {
		violations, err := detectTSMockViolations(opts.Root, opts.Cfg)
		if err != nil {
			return nil, fmt.Errorf("scan mocks: %w", err)
		}
		for _, v := range violations {
			result.Issues = append(result.Issues, report.Issue{
				Type:     "mock",
				Severity: "error",
				Message:  fmt.Sprintf("over-mocking: real dep %q with mock lib %q", v.RealDep, v.MockLib),
				File:     v.File,
				Line:     v.Line,
				Detail:   v.RealDep,
			})
		}
	}

	covPath := filepath.Join(opts.Root, opts.Cfg.Coverage.UnitPath)
	rep, err := coverage.ParseTS(covPath)
	if err == nil {
		result.Coverage = rep.Percent
	}

	if result.Coverage < float64(result.Threshold) {
		result.Issues = append(result.Issues, report.Issue{
			Type:     "coverage",
			Severity: "error",
			Message:  fmt.Sprintf("coverage %.2f%% below threshold %d%%", result.Coverage, result.Threshold),
			Value:    result.Coverage,
		})
	}

	result.Passed = len(result.Issues) == 0
	return result, nil
}

func runShell(dir, command string) error {
	if command == "" {
		return nil
	}
	c := exec.Command("sh", "-c", command)
	c.Dir = dir
	c.Stdout = io.Discard
	c.Stderr = io.Discard
	return c.Run()
}

func fixIssue(ctx context.Context, client *ai.Client, p *patcher.Patcher, progress Progress, opts Options, language string, issue report.Issue) error {
	if opts.DryRun {
		fmt.Fprintf(opts.Out, "  → would fix %s (%s)\n", issue.File, issue.Message)
		return nil
	}

	content, err := os.ReadFile(issue.File)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	rule := "remove over-mocking and use real dependency in integration tests"
	if issue.Type == "coverage" || issue.Type == "gap" {
		rule = "add tests to increase coverage of uncovered branches"
	}

	progress.StreamStarted("ai")
	diff, err := client.FixStream(ctx, issue.Message, issue.Type, rule, issue.File, language, string(content),
		func(chunk string) { progress.StreamChunk(chunk) },
	)
	progress.StreamFinished()
	if err != nil {
		return fmt.Errorf("ai: %w", err)
	}
	if diff == "" {
		return fmt.Errorf("empty response from ai")
	}

	if err := p.Backup(issue.File); err != nil {
		return fmt.Errorf("backup: %w", err)
	}

	if err := p.Apply(issue.File, diff); err != nil {
		_ = p.Rollback(issue.File)
		return fmt.Errorf("apply: %w", err)
	}

	testCmd := opts.Cfg.Coverage.UnitCommand
	if testCmd == "" {
		fmt.Fprintf(opts.Out, "  ✓ fixed %s (no test command, skipped verification)\n", issue.File)
		return nil
	}

	if err := runShell(opts.Root, testCmd); err != nil {
		_ = p.Rollback(issue.File)
		return fmt.Errorf("tests failed after fix, rolled back: %w", err)
	}

	fmt.Fprintf(opts.Out, "  ✓ fixed %s (tests passed)\n", issue.File)
	return nil
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
