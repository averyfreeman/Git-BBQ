package gitbbq

import (
	"fmt"
	"os"
	"path/filepath"
)

func ValidateProject(root string) error {
	manifest, err := loadManifest(root)
	if err != nil {
		return err
	}
	if _, err := loadGitHabits(root); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(root, ContextFilename)); err != nil {
		return fmt.Errorf("missing %s: %w", ContextFilename, err)
	}
	if _, err := ReadHookConfig(root); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(root, MattDependencyMetadataPath)); err != nil {
		return fmt.Errorf("missing %s: %w", MattDependencyMetadataPath, err)
	}
	var dependency MattDependency
	if err := readYAML(filepath.Join(root, MattDependencyMetadataPath), &dependency); err != nil {
		return err
	}
	if dependency != manifest.Matt {
		return fmt.Errorf("Matt dependency metadata does not match the manifest pin")
	}
	if _, err := os.Stat(filepath.Join(root, ContextMapFilename)); err != nil {
		return fmt.Errorf("missing %s: %w", ContextMapFilename, err)
	}
	if _, err := readOwnershipLedger(root); err != nil {
		return fmt.Errorf("invalid %s: %w", OwnershipFilename, err)
	}
	for _, path := range listADRPaths(root) {
		if _, err := ParseADR(path); err != nil {
			return err
		}
	}
	return nil
}
