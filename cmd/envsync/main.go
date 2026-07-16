package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/Theoabw/envsync/internal/envsync"
)

var version = "dev"

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ", ") }
func (s *stringList) Set(value string) error {
	if value == "" {
		return errors.New("pattern cannot be empty")
	}
	*s = append(*s, value)
	return nil
}

type config struct {
	dir               string
	recursive         bool
	matches           stringList
	excludes          stringList
	noDefaultExcludes bool
	example           string
	env               string
	keepExtra         bool
	followSymlink     bool
	dryRun            bool
	check             bool
	backup            bool
	quiet             bool
	color             string
	showVersion       bool
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	cfg := config{}
	fs := flag.NewFlagSet("envsync", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&cfg.dir, "dir", ".", "directory to scan and base for relative paths")
	fs.BoolVar(&cfg.recursive, "recursive", false, "scan nested directories")
	fs.Var(&cfg.matches, "match", "include glob; repeatable (replaces default patterns)")
	fs.Var(&cfg.excludes, "exclude", "exclude glob; repeatable")
	fs.BoolVar(&cfg.noDefaultExcludes, "no-default-excludes", false, "scan dependency/vendor directories during recursive discovery")
	fs.StringVar(&cfg.example, "example", "", "sync one exact example file")
	fs.StringVar(&cfg.env, "env", "", "destination for --example (default: remove .example suffix)")
	fs.BoolVar(&cfg.keepExtra, "keep-extra", false, "keep newly discovered extra keys active")
	fs.BoolVar(&cfg.followSymlink, "follow-symlink", false, "update a destination symlink's resolved target")
	fs.BoolVar(&cfg.dryRun, "dry-run", false, "show redacted changes without writing")
	fs.BoolVar(&cfg.check, "check", false, "exit 1 if synchronization is needed")
	fs.BoolVar(&cfg.backup, "backup", false, "create a timestamped backup before replacing an existing file")
	fs.BoolVar(&cfg.quiet, "quiet", false, "suppress normal output")
	fs.StringVar(&cfg.color, "color", "auto", "color output: auto, always, or never")
	fs.BoolVar(&cfg.showVersion, "version", false, "print version and exit")
	fs.Usage = func() { printHelp(stderr, fs) }
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "envsync: unexpected arguments: %s\n", strings.Join(fs.Args(), " "))
		return 2
	}
	if cfg.showVersion {
		fmt.Fprintf(stdout, "envsync %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
		return 0
	}
	if cfg.dryRun && cfg.check {
		fmt.Fprintln(stderr, "envsync: --dry-run and --check cannot be combined")
		return 2
	}
	if cfg.backup && (cfg.dryRun || cfg.check) {
		fmt.Fprintln(stderr, "envsync: --backup has no effect with --dry-run or --check")
		return 2
	}
	if cfg.color != "auto" && cfg.color != "always" && cfg.color != "never" {
		fmt.Fprintln(stderr, "envsync: --color must be auto, always, or never")
		return 2
	}
	if err := validatePatterns(append(append([]string(nil), cfg.matches...), cfg.excludes...)); err != nil {
		fmt.Fprintf(stderr, "envsync: %v\n", err)
		return 2
	}

	pairs, err := envsync.Discover(envsync.DiscoverOptions{
		Dir: cfg.dir, Recursive: cfg.recursive, Matches: cfg.matches, Excludes: cfg.excludes,
		NoDefaultExcludes: cfg.noDefaultExcludes, Example: cfg.example, Env: cfg.env,
	})
	if err != nil {
		fmt.Fprintf(stderr, "envsync: %v\n", err)
		return 2
	}
	if len(pairs) == 0 {
		fmt.Fprintf(stderr, "envsync: no example files found in %s\n", cfg.dir)
		return 2
	}

	plans := make([]envsync.Plan, 0, len(pairs))
	for _, pair := range pairs {
		plan, err := envsync.Analyze(pair, envsync.FileOptions{KeepExtra: cfg.keepExtra, FollowSymlink: cfg.followSymlink})
		if err != nil {
			fmt.Fprintf(stderr, "envsync: %v\n", err)
			return 2
		}
		plans = append(plans, plan)
	}
	reporter := newReporter(stdout, cfg.color, cfg.quiet)
	changed := 0
	for _, plan := range plans {
		if plan.Result.Changed {
			changed++
		}
		reporter.plan(plan, cfg.dryRun || cfg.check)
	}
	if cfg.check {
		if changed > 0 {
			reporter.summary(fmt.Sprintf("%d file(s) need synchronization", changed), "warn")
			return 1
		}
		reporter.summary("all environment files are synchronized", "ok")
		return 0
	}
	if cfg.dryRun {
		reporter.summary(fmt.Sprintf("dry run: %d file(s) would change", changed), statusForCount(changed))
		return 0
	}
	for _, plan := range plans {
		if !plan.Result.Changed {
			continue
		}
		backupPath, err := envsync.Apply(plan, cfg.backup)
		if err != nil {
			fmt.Fprintf(stderr, "envsync: %v\n", err)
			fmt.Fprintln(stderr, "envsync: earlier files, if any, were already updated")
			return 2
		}
		if backupPath != "" {
			reporter.note("backup", backupPath)
		}
	}
	reporter.summary(fmt.Sprintf("synchronized %d file(s); %d unchanged", changed, len(plans)-changed), "ok")
	return 0
}

func validatePatterns(patterns []string) error {
	for _, pattern := range patterns {
		if _, err := filepath.Match(filepath.FromSlash(pattern), "x"); err != nil {
			return fmt.Errorf("invalid glob %q: %w", pattern, err)
		}
	}
	return nil
}

func printHelp(w io.Writer, fs *flag.FlagSet) {
	fmt.Fprintln(w, `envsync synchronizes dotenv files from .env.example templates without
evaluating or printing their values.

Usage:
  envsync [options]
  envsync --example PATH [--env PATH] [options]

Discovery:
  By default, the current directory is scanned for .env.example and
  .env.<name>.example. Every match is synchronized. Use --match/--exclude to
  limit the set, or --recursive to scan a monorepo. Quote shell globs.

Merge policy:
  Existing values win and are reordered to match the example. Missing keys use
  example defaults. Extra active keys are reversibly commented out unless
  --keep-extra is set. If a disabled key returns, its old value is restored.

Safety:
  Files are validated before any write. Replacements are atomic per file,
  existing modes are preserved, new files are private on Unix, and displayed
  actions never include values. Use --dry-run for a preview or --check in CI.

Options:`)
	fs.PrintDefaults()
	fmt.Fprintln(w, `
Exit codes:
  0  success (including dry-run)
  1  --check found files that need synchronization
  2  usage, parsing, discovery, or I/O error

Examples:
  envsync
  envsync --dir ./services/api --dry-run
  envsync --recursive --match '.env.production.example'
  envsync --example .env.local.example --env .env.local --keep-extra
  envsync --check --color never`)
}

type reporter struct {
	w     io.Writer
	color bool
	quiet bool
}

func newReporter(w io.Writer, mode string, quiet bool) reporter {
	color := mode == "always"
	if mode == "auto" && os.Getenv("NO_COLOR") == "" {
		if file, ok := w.(*os.File); ok {
			if info, err := file.Stat(); err == nil {
				color = info.Mode()&os.ModeCharDevice != 0
			}
		}
	}
	return reporter{w: w, color: color, quiet: quiet}
}

func (r reporter) plan(plan envsync.Plan, preview bool) {
	if r.quiet {
		return
	}
	state := "unchanged"
	if plan.Result.Changed {
		state = "sync"
		if preview {
			state = "would sync"
		}
	}
	exampleName, envName := plan.Pair.Rel, plan.Pair.EnvRel
	if exampleName == "" {
		exampleName = plan.Pair.Example
	}
	if envName == "" {
		envName = plan.Pair.Env
	}
	fmt.Fprintf(r.w, "%s %s -> %s\n", r.paint(iconFor(state), colorFor(state)), exampleName, envName)
	if !plan.Result.Changed {
		return
	}
	actions := append([]envsync.Action(nil), plan.Result.Actions...)
	sort.SliceStable(actions, func(i, j int) bool {
		return actionRank(actions[i].Kind) < actionRank(actions[j].Kind)
	})
	for _, action := range actions {
		if action.Kind == envsync.ActionPreserved {
			continue
		}
		fmt.Fprintf(r.w, "  %s %-9s %s\n", r.paint(iconFor(string(action.Kind)), colorFor(string(action.Kind))), action.Kind, action.Key)
	}
	if !plan.EnvExists {
		fmt.Fprintln(r.w, r.paint("  ! review the copied defaults before running the application", "yellow"))
	}
}

func (r reporter) note(label, value string) {
	if !r.quiet {
		fmt.Fprintf(r.w, "  %s %s: %s\n", r.paint("•", "cyan"), label, value)
	}
}

func (r reporter) summary(message, status string) {
	if !r.quiet {
		fmt.Fprintf(r.w, "%s %s\n", r.paint(iconFor(status), colorFor(status)), message)
	}
}

func (r reporter) paint(text, color string) string {
	if !r.color {
		return text
	}
	codes := map[string]string{"green": "32", "yellow": "33", "red": "31", "cyan": "36", "dim": "2"}
	if code := codes[color]; code != "" {
		return "\x1b[" + code + "m" + text + "\x1b[0m"
	}
	return text
}

func iconFor(status string) string {
	switch status {
	case "ok", "unchanged", "preserved":
		return "✓"
	case "warn", "would sync", "added", "restored":
		return "+"
	case "disabled":
		return "-"
	case "kept":
		return "="
	default:
		return "→"
	}
}

func colorFor(status string) string {
	switch status {
	case "ok", "unchanged", "preserved":
		return "green"
	case "warn", "would sync", "added", "restored":
		return "yellow"
	case "disabled":
		return "red"
	case "kept":
		return "cyan"
	default:
		return "cyan"
	}
}

func actionRank(kind envsync.ActionKind) int {
	switch kind {
	case envsync.ActionAdded:
		return 0
	case envsync.ActionRestored:
		return 1
	case envsync.ActionDisabled:
		return 2
	case envsync.ActionKept:
		return 3
	default:
		return 4
	}
}

func statusForCount(count int) string {
	if count == 0 {
		return "ok"
	}
	return "warn"
}
