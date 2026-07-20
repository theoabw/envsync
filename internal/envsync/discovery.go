package envsync

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

var defaultMatches = []string{".env.example", ".env.*.example"}

var defaultExcludedDirs = map[string]struct{}{
	".git": {}, ".hg": {}, ".svn": {}, "node_modules": {}, "vendor": {},
}

type Pair struct {
	Example string
	Env     string
	Rel     string
	EnvRel  string
}

type DiscoverOptions struct {
	Dir               string
	Recursive         bool
	Matches           []string
	Excludes          []string
	NoDefaultExcludes bool
	Example           string
	Env               string
}

func Discover(opts DiscoverOptions) ([]Pair, error) {
	root, err := filepath.Abs(opts.Dir)
	if err != nil {
		return nil, err
	}
	if opts.Example != "" || opts.Env != "" {
		if opts.Example == "" {
			return nil, fmt.Errorf("--env requires --example")
		}
		if opts.Recursive || len(opts.Matches) > 0 || len(opts.Excludes) > 0 {
			return nil, fmt.Errorf("--example mode cannot be combined with discovery filters or --recursive")
		}
		example := resolveFrom(root, opts.Example)
		env := opts.Env
		if env == "" {
			if !strings.HasSuffix(example, ".example") {
				return nil, fmt.Errorf("cannot derive destination from %q; provide --env", opts.Example)
			}
			env = strings.TrimSuffix(example, ".example")
		} else {
			env = resolveFrom(root, env)
		}
		return []Pair{{Example: filepath.Clean(example), Env: filepath.Clean(env), Rel: displayPath(root, example), EnvRel: displayPath(root, env)}}, nil
	}

	patterns := opts.Matches
	if len(patterns) == 0 {
		patterns = defaultMatches
	}
	var examples []string
	walk := func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path == root {
				return nil
			}
			if !opts.Recursive {
				return filepath.SkipDir
			}
			if !opts.NoDefaultExcludes {
				if _, skip := defaultExcludedDirs[entry.Name()]; skip {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if entry.Type()&fs.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if matchesAny(patterns, rel) && !matchesAny(opts.Excludes, rel) {
			examples = append(examples, path)
		}
		return nil
	}
	if err := filepath.WalkDir(root, walk); err != nil {
		return nil, fmt.Errorf("discover examples: %w", err)
	}
	sort.Strings(examples)
	pairs := make([]Pair, 0, len(examples))
	targets := make(map[string]string)
	exampleSet := make(map[string]struct{}, len(examples))
	for _, path := range examples {
		exampleSet[filepath.Clean(path)] = struct{}{}
	}
	for _, example := range examples {
		env := strings.TrimSuffix(example, ".example")
		if env == example {
			return nil, fmt.Errorf("matched example %q does not end in .example", displayPath(root, example))
		}
		cleanEnv := filepath.Clean(env)
		if previous, exists := targets[cleanEnv]; exists {
			return nil, fmt.Errorf("examples %q and %q resolve to the same destination", previous, example)
		}
		if _, isExample := exampleSet[cleanEnv]; isExample {
			return nil, fmt.Errorf("destination %q is also a discovered example", displayPath(root, cleanEnv))
		}
		targets[cleanEnv] = example
		pairs = append(pairs, Pair{Example: example, Env: cleanEnv, Rel: displayPath(root, example), EnvRel: displayPath(root, cleanEnv)})
	}
	return pairs, nil
}

func matchesAny(patterns []string, rel string) bool {
	for _, pattern := range patterns {
		candidate := rel
		pattern = filepath.ToSlash(pattern)
		if !strings.Contains(pattern, "/") {
			candidate = filepath.Base(filepath.FromSlash(rel))
		}
		matched, err := filepath.Match(filepath.FromSlash(pattern), filepath.FromSlash(candidate))
		if err == nil && matched {
			return true
		}
	}
	return false
}

func resolveFrom(root, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

func displayPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(rel)
	}
	return path
}
