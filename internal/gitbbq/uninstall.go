package gitbbq

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func writeOwnershipLedger(root string, created []string, force bool) (bool, error) {
	path := filepath.Join(root, filepath.FromSlash(OwnershipFilename))
	files := make(map[string]string, len(created))
	existing := false
	if _, err := os.Lstat(path); err == nil {
		ledger, readErr := readOwnershipLedger(root)
		if readErr != nil {
			return false, readErr
		}
		for relative, digest := range ledger.Files {
			files[relative] = digest
		}
		existing = true
	} else if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	changed := false
	for _, relative := range created {
		if relative == OwnershipFilename {
			continue
		}
		clean, err := safeGeneratedPath(relative)
		if err != nil {
			return false, err
		}
		path, err := ensureSafeGeneratedTarget(root, clean)
		if err != nil {
			return false, err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return false, err
		}
		if !info.Mode().IsRegular() {
			return false, fmt.Errorf("owned path is not regular: %s", clean)
		}
		digest, err := hashFile(path)
		if err != nil {
			return false, err
		}
		if files[clean] != digest {
			changed = true
		}
		files[clean] = digest
	}
	if existing && !changed {
		return false, nil
	}
	return writeOwnershipData(root, files, force || existing)
}

func recordGeneratedOwnership(root string, created []string) error {
	if len(created) == 0 {
		return nil
	}
	ledger, err := readOwnershipLedger(root)
	if err != nil {
		return err
	}
	files := make(map[string]string, len(ledger.Files)+len(created))
	for relative, digest := range ledger.Files {
		files[relative] = digest
	}
	changed := false
	for _, relative := range created {
		if relative == OwnershipFilename {
			continue
		}
		clean, err := safeGeneratedPath(relative)
		if err != nil {
			return err
		}
		path, err := ensureSafeGeneratedTarget(root, clean)
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("owned path is not regular: %s", clean)
		}
		digest, err := hashFile(path)
		if err != nil {
			return err
		}
		if files[clean] != digest {
			files[clean] = digest
			changed = true
		}
	}
	if !changed {
		return nil
	}
	_, err = writeOwnershipData(root, files, true)
	return err
}

func recordExistingOwnership(root string, created []string) error {
	if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(OwnershipFilename))); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	return recordGeneratedOwnership(root, created)
}

func writeOwnershipData(root string, files map[string]string, force bool) (bool, error) {
	data, err := json.MarshalIndent(OwnershipLedger{Version: 1, Files: files}, "", "  ")
	if err != nil {
		return false, err
	}
	data = append(data, '\n')
	return writeGenerated(root, OwnershipFilename, data, force)
}

// AssessUninstall reports only unchanged files owned by Git BBQ as removable.
// Modified, symlinked, and non-regular paths are conflicts and are preserved.
func AssessUninstall(root string) (UninstallAssessment, error) {
	root = filepath.Clean(root)
	if root == "." {
		root, _ = os.Getwd()
	}
	ledger, err := readOwnershipLedger(root)
	if err != nil {
		return UninstallAssessment{}, err
	}
	assessment := UninstallAssessment{Mode: "uninstall-assessment", Root: root, Removable: []string{}, Conflicts: []string{}, Missing: []string{}, ReadOnly: true}
	paths := make([]string, 0, len(ledger.Files))
	for relative := range ledger.Files {
		paths = append(paths, relative)
	}
	sort.Strings(paths)
	for _, relative := range paths {
		clean, err := safeGeneratedPath(relative)
		if err != nil {
			assessment.Conflicts = append(assessment.Conflicts, relative)
			continue
		}
		path, err := ensureSafeGeneratedTarget(root, clean)
		if err != nil {
			assessment.Conflicts = append(assessment.Conflicts, clean)
			continue
		}
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			assessment.Missing = append(assessment.Missing, clean)
			continue
		}
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			assessment.Conflicts = append(assessment.Conflicts, clean)
			continue
		}
		digest, err := hashFile(path)
		if err != nil || digest != ledger.Files[relative] {
			assessment.Conflicts = append(assessment.Conflicts, clean)
			continue
		}
		assessment.Removable = append(assessment.Removable, clean)
	}
	return assessment, nil
}

// Uninstall removes only unchanged files recorded in the ownership ledger.
// It never follows symlinks, removes user-modified files, or touches legacy
// project artifacts outside the ledger.
func Uninstall(root string) (UninstallResult, error) {
	assessment, err := AssessUninstall(root)
	if err != nil {
		return UninstallResult{}, err
	}
	result := UninstallResult{Root: assessment.Root, Removed: []string{}, Preserved: append([]string(nil), assessment.Conflicts...), Missing: append([]string(nil), assessment.Missing...)}
	ledger, err := readOwnershipLedger(assessment.Root)
	if err != nil {
		return result, err
	}
	for _, relative := range assessment.Removable {
		clean, err := safeGeneratedPath(relative)
		if err != nil {
			appendUnique(&result.Preserved, relative)
			continue
		}
		path, err := ensureSafeGeneratedTarget(assessment.Root, clean)
		if err != nil {
			appendUnique(&result.Preserved, clean)
			continue
		}
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			appendUnique(&result.Preserved, clean)
			continue
		}
		digest, err := hashFile(path)
		if err != nil || digest != ledger.Files[clean] {
			appendUnique(&result.Preserved, clean)
			continue
		}
		if err := os.Remove(path); err != nil {
			return result, fmt.Errorf("remove %s: %w", relative, err)
		}
		result.Removed = append(result.Removed, clean)
	}
	ledgerPath := filepath.Join(assessment.Root, filepath.FromSlash(OwnershipFilename))
	if len(result.Preserved) > 0 {
		appendUnique(&result.Preserved, OwnershipFilename)
	} else if _, err := ensureSafeGeneratedTarget(assessment.Root, OwnershipFilename); err != nil {
		appendUnique(&result.Preserved, OwnershipFilename)
	} else if err := os.Remove(ledgerPath); err == nil {
		result.Removed = append(result.Removed, OwnershipFilename)
	} else if !os.IsNotExist(err) {
		return result, fmt.Errorf("remove %s: %w", OwnershipFilename, err)
	}
	sort.Strings(result.Removed)
	sort.Strings(result.Preserved)
	sort.Strings(result.Missing)
	return result, nil
}

func readOwnershipLedger(root string) (OwnershipLedger, error) {
	path, err := ensureSafeGeneratedTarget(root, OwnershipFilename)
	if err != nil {
		return OwnershipLedger{}, fmt.Errorf("read %s: %w", OwnershipFilename, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return OwnershipLedger{}, fmt.Errorf("read %s: %w", OwnershipFilename, err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var ledger OwnershipLedger
	if err := decoder.Decode(&ledger); err != nil {
		return OwnershipLedger{}, fmt.Errorf("parse %s: %w", OwnershipFilename, err)
	}
	if ledger.Version != 1 {
		return OwnershipLedger{}, fmt.Errorf("unsupported ownership ledger version %d", ledger.Version)
	}
	if ledger.Files == nil {
		return OwnershipLedger{}, fmt.Errorf("ownership ledger files are required")
	}
	for relative := range ledger.Files {
		clean, err := safeGeneratedPath(relative)
		if err != nil || clean != relative {
			return OwnershipLedger{}, fmt.Errorf("ownership ledger contains unsafe path %q", relative)
		}
	}
	return ledger, nil
}

func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func safeGeneratedPath(relative string) (string, error) {
	if relative == "" || strings.Contains(relative, "\\") || strings.HasPrefix(relative, "/") || (len(relative) >= 2 && relative[1] == ':') {
		return "", fmt.Errorf("refusing unsafe owned path %q", relative)
	}
	parts := strings.Split(relative, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("refusing unsafe owned path %q", relative)
		}
	}
	clean := filepath.Clean(filepath.FromSlash(relative))
	if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("refusing unsafe owned path %q", relative)
	}
	return filepath.ToSlash(clean), nil
}

func appendUnique(values *[]string, value string) {
	for _, existing := range *values {
		if existing == value {
			return
		}
	}
	*values = append(*values, value)
}
