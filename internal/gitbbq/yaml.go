package gitbbq

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

func readYAML(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

func marshalYAML(value any) ([]byte, error) {
	data, err := yaml.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal YAML: %w", err)
	}
	return data, nil
}

func writeGenerated(root, relative string, data []byte, force bool) (bool, error) {
	clean, err := safeGeneratedPath(relative)
	if err != nil {
		return false, fmt.Errorf("refusing unsafe generated path %q: %w", relative, err)
	}
	path, err := ensureSafeGeneratedTarget(root, clean)
	if err != nil {
		return false, err
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return false, fmt.Errorf("refusing to write symlink %s", relative)
		}
		if !info.Mode().IsRegular() {
			return false, fmt.Errorf("refusing to write non-regular path %s", relative)
		}
		if !force {
			return false, nil
		}
	} else if !os.IsNotExist(err) {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".gitbbq-*")
	if err != nil {
		return false, err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		return false, err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return false, err
	}
	if err := temporary.Close(); err != nil {
		return false, err
	}
	if err := os.Rename(temporaryName, path); err != nil {
		return false, err
	}
	return true, nil
}

func ensureSafeGeneratedTarget(root, relative string) (string, error) {
	clean, err := safeGeneratedPath(relative)
	if err != nil {
		return "", err
	}
	rootAbsolute, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve generated root: %w", err)
	}
	rootInfo, err := os.Stat(rootAbsolute)
	if err != nil {
		return "", fmt.Errorf("stat generated root: %w", err)
	}
	if !rootInfo.IsDir() {
		return "", fmt.Errorf("generated root is not a directory: %s", root)
	}
	resolvedRoot, err := filepath.EvalSymlinks(rootAbsolute)
	if err != nil {
		return "", fmt.Errorf("resolve generated root: %w", err)
	}
	path := filepath.Join(rootAbsolute, filepath.FromSlash(clean))
	current := rootAbsolute
	expected := resolvedRoot
	parts := strings.Split(filepath.ToSlash(clean), "/")
	for index, part := range parts {
		current = filepath.Join(current, filepath.FromSlash(part))
		expected = filepath.Join(expected, filepath.FromSlash(part))
		info, statErr := os.Lstat(current)
		if os.IsNotExist(statErr) {
			break
		}
		if statErr != nil {
			return "", fmt.Errorf("inspect generated path %s: %w", clean, statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("refusing generated path through symlink: %s", clean)
		}
		if index < len(parts)-1 && !info.IsDir() {
			return "", fmt.Errorf("generated path parent is not a directory: %s", clean)
		}
		resolved, resolveErr := filepath.EvalSymlinks(current)
		if resolveErr != nil {
			return "", fmt.Errorf("resolve generated path %s: %w", clean, resolveErr)
		}
		if !sameFilesystemPath(resolved, expected) {
			return "", fmt.Errorf("refusing generated path through reparse point: %s", clean)
		}
		if index == len(parts)-1 && !info.Mode().IsRegular() {
			return "", fmt.Errorf("refusing generated path with non-regular target: %s", clean)
		}
	}
	return path, nil
}

func sameFilesystemPath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func loadManifest(root string) (Manifest, error) {
	var manifest Manifest
	if err := readYAML(filepath.Join(root, ManifestFilename), &manifest); err != nil {
		return Manifest{}, err
	}
	if err := ValidateManifest(manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func ReadManifest(root string) (Manifest, error) {
	return loadManifest(root)
}

func loadGitHabits(root string) (GitHabits, error) {
	var config GitHabits
	if err := readYAML(filepath.Join(root, GitHabitsFilename), &config); err != nil {
		return GitHabits{}, err
	}
	if err := ValidateGitHabits(config); err != nil {
		return GitHabits{}, err
	}
	return config, nil
}

func ReadGitHabits(root string) (GitHabits, error) {
	return loadGitHabits(root)
}
