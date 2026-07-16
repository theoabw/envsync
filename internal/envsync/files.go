package envsync

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type FileOptions struct {
	KeepExtra     bool
	FollowSymlink bool
}

type Plan struct {
	Pair      Pair
	Result    MergeResult
	EnvExists bool
	EnvPath   string
	EnvMode   os.FileMode
	EnvHash   [sha256.Size]byte
}

func Analyze(pair Pair, opts FileOptions) (Plan, error) {
	exampleData, err := os.ReadFile(pair.Example)
	if err != nil {
		return Plan{}, fmt.Errorf("read example %s: %w", pair.Example, err)
	}
	exampleInfo, err := os.Stat(pair.Example)
	if err != nil {
		return Plan{}, fmt.Errorf("stat example %s: %w", pair.Example, err)
	}
	if !exampleInfo.Mode().IsRegular() {
		return Plan{}, fmt.Errorf("example is not a regular file: %s", pair.Example)
	}

	envPath := pair.Env
	lstat, err := os.Lstat(envPath)
	envExists := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Plan{}, fmt.Errorf("inspect destination %s: %w", envPath, err)
	}
	if envExists && lstat.Mode()&os.ModeSymlink != 0 {
		if !opts.FollowSymlink {
			return Plan{}, fmt.Errorf("destination is a symlink (use --follow-symlink to update its target): %s", envPath)
		}
		resolved, err := filepath.EvalSymlinks(envPath)
		if err != nil {
			return Plan{}, fmt.Errorf("resolve destination symlink %s: %w", envPath, err)
		}
		envPath, err = filepath.Abs(resolved)
		if err != nil {
			return Plan{}, err
		}
		lstat, err = os.Stat(envPath)
		if err != nil {
			return Plan{}, fmt.Errorf("stat destination target %s: %w", envPath, err)
		}
	}
	var envData []byte
	mode := os.FileMode(0o600)
	if envExists {
		if !lstat.Mode().IsRegular() {
			return Plan{}, fmt.Errorf("destination is not a regular file: %s", envPath)
		}
		envData, err = os.ReadFile(envPath)
		if err != nil {
			return Plan{}, fmt.Errorf("read destination %s: %w", envPath, err)
		}
		mode = lstat.Mode().Perm()
	}
	result, err := Merge(pair.Example, envPath, exampleData, envData, envExists, MergeOptions{KeepExtra: opts.KeepExtra})
	if err != nil {
		return Plan{}, err
	}
	return Plan{
		Pair: pair, Result: result, EnvExists: envExists, EnvPath: envPath,
		EnvMode: mode, EnvHash: sha256.Sum256(envData),
	}, nil
}

func Apply(plan Plan, backup bool) (string, error) {
	if !plan.Result.Changed {
		return "", nil
	}
	if plan.EnvExists {
		current, err := os.ReadFile(plan.EnvPath)
		if err != nil {
			return "", fmt.Errorf("re-read destination %s: %w", plan.EnvPath, err)
		}
		if sha256.Sum256(current) != plan.EnvHash {
			return "", fmt.Errorf("destination changed after analysis; refusing to overwrite: %s", plan.EnvPath)
		}
	} else {
		if _, err := os.Lstat(plan.EnvPath); err == nil {
			return "", fmt.Errorf("destination appeared after analysis; refusing to overwrite: %s", plan.EnvPath)
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("recheck destination %s: %w", plan.EnvPath, err)
		}
	}
	var backupPath string
	if backup && plan.EnvExists {
		var err error
		backupPath, err = writeBackup(plan.EnvPath, plan.EnvMode)
		if err != nil {
			return "", err
		}
	}
	if err := atomicWrite(plan.EnvPath, plan.Result.Content, plan.EnvMode); err != nil {
		return backupPath, err
	}
	return backupPath, nil
}

func writeBackup(path string, mode os.FileMode) (string, error) {
	suffix := time.Now().UTC().Format("20060102T150405.000000000Z")
	backupPath := path + ".bak." + suffix
	source, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open backup source: %w", err)
	}
	defer source.Close()
	dest, err := os.OpenFile(backupPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return "", fmt.Errorf("create backup %s: %w", backupPath, err)
	}
	ok := false
	defer func() {
		_ = dest.Close()
		if !ok {
			_ = os.Remove(backupPath)
		}
	}()
	if _, err := io.Copy(dest, source); err != nil {
		return "", fmt.Errorf("write backup %s: %w", backupPath, err)
	}
	if err := dest.Sync(); err != nil {
		return "", fmt.Errorf("sync backup %s: %w", backupPath, err)
	}
	if err := dest.Close(); err != nil {
		return "", fmt.Errorf("close backup %s: %w", backupPath, err)
	}
	ok = true
	return backupPath, nil
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".envsync-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	keep := false
	defer func() {
		_ = tmp.Close()
		if !keep {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(mode); err != nil && runtime.GOOS != "windows" {
		return fmt.Errorf("set temporary file permissions: %w", err)
	}
	if _, err := io.Copy(tmp, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := replaceFile(tmpPath, path); err != nil {
		return fmt.Errorf("replace destination %s: %w", path, err)
	}
	keep = true
	if err := syncDirectory(dir); err != nil {
		return fmt.Errorf("sync destination directory: %w", err)
	}
	return nil
}
