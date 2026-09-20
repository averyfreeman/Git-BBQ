package gitbbq

import (
	"fmt"
	"strconv"
	"strings"
)

type semanticVersion struct {
	major      uint64
	minor      uint64
	patch      uint64
	prerelease []string
}

// ValidateTagProgression rejects a candidate that does not advance observed SemVer tags.
// Non-SemVer tags are ignored because repositories may contain non-release tags.
func ValidateTagProgression(existingTags []string, candidate string) error {
	candidateVersion, err := parseSemanticVersion(candidate)
	if err != nil {
		return err
	}
	for _, existing := range existingTags {
		existing = strings.TrimSpace(existing)
		if existing == "" || !safeSemanticVersionTag(existing) {
			continue
		}
		existingVersion, err := parseSemanticVersion(existing)
		if err != nil {
			continue
		}
		if compareSemanticVersions(candidateVersion, existingVersion) <= 0 {
			return fmt.Errorf("tag %q is not newer than existing tag %q", candidate, existing)
		}
	}
	return nil
}

func parseSemanticVersion(value string) (semanticVersion, error) {
	value = strings.TrimSpace(value)
	if !safeSemanticVersionTag(value) {
		return semanticVersion{}, fmt.Errorf("tag is not a valid semver value: %q", value)
	}
	value = strings.TrimPrefix(value, "v")
	if buildIndex := strings.IndexByte(value, '+'); buildIndex >= 0 {
		value = value[:buildIndex]
	}
	prerelease := []string(nil)
	if prereleaseIndex := strings.IndexByte(value, '-'); prereleaseIndex >= 0 {
		prerelease = strings.Split(value[prereleaseIndex+1:], ".")
		value = value[:prereleaseIndex]
	}
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return semanticVersion{}, fmt.Errorf("tag is not a valid semver value: %q", value)
	}
	major, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return semanticVersion{}, fmt.Errorf("tag is not a valid semver value: %q", value)
	}
	minor, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return semanticVersion{}, fmt.Errorf("tag is not a valid semver value: %q", value)
	}
	patch, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		return semanticVersion{}, fmt.Errorf("tag is not a valid semver value: %q", value)
	}
	return semanticVersion{major: major, minor: minor, patch: patch, prerelease: prerelease}, nil
}

func compareSemanticVersions(left, right semanticVersion) int {
	for _, pair := range [][2]uint64{{left.major, right.major}, {left.minor, right.minor}, {left.patch, right.patch}} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}
	if len(left.prerelease) == 0 && len(right.prerelease) > 0 {
		return 1
	}
	if len(left.prerelease) > 0 && len(right.prerelease) == 0 {
		return -1
	}
	for index := 0; index < len(left.prerelease) && index < len(right.prerelease); index++ {
		leftIdentifier := left.prerelease[index]
		rightIdentifier := right.prerelease[index]
		leftNumber, leftIsNumber := parsePrereleaseNumber(leftIdentifier)
		rightNumber, rightIsNumber := parsePrereleaseNumber(rightIdentifier)
		if leftIsNumber && rightIsNumber {
			if leftNumber < rightNumber {
				return -1
			}
			if leftNumber > rightNumber {
				return 1
			}
			continue
		}
		if leftIsNumber != rightIsNumber {
			if leftIsNumber {
				return -1
			}
			return 1
		}
		if leftIdentifier < rightIdentifier {
			return -1
		}
		if leftIdentifier > rightIdentifier {
			return 1
		}
	}
	if len(left.prerelease) < len(right.prerelease) {
		return -1
	}
	if len(left.prerelease) > len(right.prerelease) {
		return 1
	}
	return 0
}

func parsePrereleaseNumber(value string) (uint64, bool) {
	if value == "" || strings.Trim(value, "0123456789") != "" {
		return 0, false
	}
	number, err := strconv.ParseUint(value, 10, 64)
	return number, err == nil
}
