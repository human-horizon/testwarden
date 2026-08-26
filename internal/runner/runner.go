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
	"github.com/HumanHorizon/testwarden/internal/errors"
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
	TUI    bool // when true, show interactive progress (fix only)
	Quiet  bool // suppress output (only exit code matters)
	Out    io.Writer
}

// RunCheck analyses the project and writes a report. Always plain text.
// Returns exit code (0 = passed, 1 = violations found).
func RunCheck(ctx context.Context, opts Options) (int, error) {
	if err := opts.validate(); err != nil {
		return 2, err
	}

	results, err := analyze(ctx, opts)
	if err != nil {
		return 1, errors.Wrap(errors.CodeFilesystem, "analysis failed", err).
			WithContext("root", opts.Root)
	}

	if !opts.Quiet {
		if opts.JSON {
			if err := report.PrintJSON(opts.Out, results); err != nil {
				return 1, errors.Wrap(errors.CodeConfig, "write report", err)
			}
		} else {
			report.PrintText(opts.Out, results)
		}
	}

	for _, r := range results {
		if !r.Passed {
			return 1, nil
		}
	}
	return 0, nil
}

// RunFix analyses the project and applies AI fixes.
// Returns exit code (0 = all fixed or nothing to fix, 1 = still has issues).
func RunFix(ctx context.Context, opts Options) (int, error) {
	if err := opts.validate(); err != nil {
		return 2, err
	}

	progress := newProgress(opts)
	if opts.TUI {
		progress.AnalyseStarted()
		defer progress.Wait()
	}

	results, err := analyze(ctx, opts)
	if err != nil {
		progress.Error(err)
		return 1, err
	}

	if !opts.Quiet && !opts.TUI {
		if opts.JSON {
			_ = report.PrintJSON(opts.Out, results)
		} else {
			report.PrintText(opts.Out, results)
		}
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
			fmt.Fprintf(opts.Out, "  ✗ %s: %v\n", item.issue.File, err)
			continue
		}
		fixed++
	}

	if !opts.Quiet {
		fmt.Fprintf(opts.Out, "\n")
		fmt.Fprintf(opts.Out, "Fixed: %d/%d issue(s)\n", fixed, len(pending))
		if fixed > 0 && fixed < len(pending) {
			fmt.Fprintf(opts.Out, "Run 'testwarden fix' again to retry remaining issues.\n")
		}
	}

	progress.Done(results)
	if fixed == len(pending) && len(pending) > 0 {
		return 0, nil
	}
	for _, r := range results {
		if !r.Passed {
			return 1, nil
		}
	}
	return 0, nil
}

// validate checks Options for required fields and provides helpful errors.
func (o Options) validate() error {
	if o.Cfg == nil {
		return errors.New(errors.CodeConfig, "config not loaded").
			WithContext("hint", "run 'testwarden init' to create .testwarden.yml")
	}
	if o.Root == "" {
		return errors.New(errors.CodeConfig, "project root is empty")
	}
	if _, err := os.Stat(o.Root); err != nil {
		if os.IsNotExist(err) {
			return errors.Wrap(errors.CodeFilesystem, "project root not found", err).
				WithContext("path", o.Root).
				WithContext("hint", "create the directory or check --root flag")
		}
		return errors.Wrap(errors.CodeFilesystem, "cannot access project root", err)
	}
	if err := config.Validate(o.Cfg); err != nil {
		return errors.Wrap(errors.CodeConfig, "invalid config", err)
	}
	return nil
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
			return nil, errors.Wrap(errors.CodeFilesystem, "scan go mocks", err)
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
	unitRep, unitErr := coverage.ParseGo(unitCovPath)
	if unitErr != nil && !os.IsNotExist(unitErr) && !isCoverageEmpty(unitErr) {
		if !opts.Quiet {
			fmt.Fprintf(opts.Out, "  (could not parse %s: %v)\n", unitCovPath, unitErr)
		}
	}
	if unitRep != nil {
		result.Coverage = unitRep.Percent
	} else if opts.Cfg.Coverage.UnitCommand != "" {
		if err := runShell(opts.Root, opts.Cfg.Coverage.UnitCommand); err != nil {
			if !opts.Quiet {
				fmt.Fprintf(opts.Out, "  (unit tests failed: %v)\n", err)
			}
		}
		if unitRep, _ = coverage.ParseGo(unitCovPath); unitRep != nil {
			result.Coverage = unitRep.Percent
		}
	}

	var integrationRep *coverage.GoReport
	if opts.Cfg.Coverage.IntegrationPath != "" {
		integrationCovPath := filepath.Join(opts.Root, opts.Cfg.Coverage.IntegrationPath)
		if opts.Cfg.Coverage.IntegrationCommand != "" {
			_ = runShell(opts.Root, opts.Cfg.Coverage.IntegrationCommand)
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

// isCoverageEmpty reports whether err means the coverage file was missing or empty.
func isCoverageEmpty(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "no such file") ||
		strings.Contains(msg, "cannot find") ||
		strings.Contains(msg, "EOF")
}

func detectGoMockViolations(root string, cfg *config.Config) ([]mocks.Violation, error) {
	if !cfg.Mocks.DetectOvermocking {
		return nil, nil
	}

	c, err := cache.New(root)
	if err != nil {
		return nil, errors.Wrap(errors.CodeFilesystem, "init cache", err)
	}
	c.UseSuffix("go")
	manifest, err := c.LoadManifest()
	if err != nil {
		return nil, errors.Wrap(errors.CodeFilesystem, "load manifest", err)
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

func detectTSMockViolations(root string, cfg *config.Config) ([]mocks.Violation, error) {
	if !cfg.Mocks.DetectOvermocking {
		return nil, nil
	}

	c, err := cache.New(root)
	if err != nil {
		return nil, errors.Wrap(errors.CodeFilesystem, "init cache", err)
	}
	c.UseSuffix("ts")
	manifest, err := c.LoadManifest()
	if err != nil {
		return nil, errors.Wrap(errors.CodeFilesystem, "load manifest", err)
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
			return nil, errors.Wrap(errors.CodeFilesystem, "scan ts mocks", err)
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
		fmt.Fprintf(opts.Out, "  → would fix %s\n", issue.File)
		return nil
	}

	content, err := os.ReadFile(issue.File)
	if err != nil {
		return errors.Wrap(errors.CodeFilesystem, "read source", err).
			WithContext("file", issue.File)
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
		return errors.Wrap(errors.CodeAI, "AI fix failed", err).
			WithContext("file", issue.File).
			WithContext("endpoint", opts.Cfg.AI.Endpoint)
	}
	if diff == "" {
		return errors.New(errors.CodeAI, "AI returned empty response").
			WithContext("file", issue.File)
	}

	if err := p.Backup(issue.File); err != nil {
		return errors.Wrap(errors.CodeFilesystem, "backup", err)
	}

	if err := p.Apply(issue.File, diff); err != nil {
		_ = p.Rollback(issue.File)
		return errors.Wrap(errors.CodeFilesystem, "apply patch", err)
	}

	testCmd := opts.Cfg.Coverage.UnitCommand
	if testCmd == "" {
		if !opts.Quiet {
			fmt.Fprintf(opts.Out, "  ✓ %s (skipped verification, no unit_command)\n", issue.File)
		}
		return nil
	}

	if err := runShell(opts.Root, testCmd); err != nil {
		_ = p.Rollback(issue.File)
		return errors.Wrap(errors.CodeValidation, "tests failed after fix (rolled back)", err).
			WithContext("file", issue.File)
	}

	if !opts.Quiet {
		fmt.Fprintf(opts.Out, "  ✓ %s\n", issue.File)
	}
	return nil
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
