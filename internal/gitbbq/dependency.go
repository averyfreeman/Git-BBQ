package gitbbq

import (
	"fmt"
	"path/filepath"
	"strings"
)

func UpdateMattDependency(root string, dependency MattDependency) (Manifest, error) {
	manifest, err := loadManifest(root)
	if err != nil {
		return Manifest{}, err
	}
	dependency.Repository = strings.TrimSpace(dependency.Repository)
	dependency.Commit = strings.TrimSpace(dependency.Commit)
	dependency.Path = strings.TrimSpace(dependency.Path)
	if dependency.Repository == "" || dependency.Commit == "" || dependency.Path == "" {
		return Manifest{}, fmt.Errorf("Matt dependency update requires repository, commit, and path")
	}
	if dependency.Path != manifest.Matt.Path {
		return Manifest{}, fmt.Errorf("Matt dependency path cannot change during an update")
	}
	manifest.Matt = dependency
	if err := ValidateManifest(manifest); err != nil {
		return Manifest{}, err
	}
	manifestData, err := marshalYAML(manifest)
	if err != nil {
		return Manifest{}, err
	}
	dependencyData, err := marshalYAML(dependency)
	if err != nil {
		return Manifest{}, err
	}
	if _, err := writeGenerated(root, ManifestFilename, manifestData, true); err != nil {
		return Manifest{}, err
	}
	if _, err := writeGenerated(root, filepath.ToSlash(MattDependencyMetadataPath), dependencyData, true); err != nil {
		return Manifest{}, err
	}
	if err := recordExistingOwnership(root, []string{ManifestFilename, MattDependencyMetadataPath}); err != nil {
		return Manifest{}, fmt.Errorf("record dependency ownership: %w", err)
	}
	return manifest, nil
}
