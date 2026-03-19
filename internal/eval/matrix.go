package eval

import (
	"fmt"
	"sort"
	"strings"
)

// ExpandMatrix computes the cartesian product of matrix dimensions, applies exclude/include.
func ExpandMatrix(dimensions map[string][]string, exclude, include []map[string]string) []map[string]string {
	keys := sortedKeys(dimensions)
	if len(keys) == 0 {
		return nil
	}

	// Check for empty dimensions
	for _, k := range keys {
		if len(dimensions[k]) == 0 {
			return nil
		}
	}

	combos := cartesian(keys, dimensions)

	// Apply excludes
	var filtered []map[string]string
	for _, combo := range combos {
		if !matchesAny(combo, exclude) {
			filtered = append(filtered, combo)
		}
	}

	// Apply includes
	for _, inc := range include {
		filtered = append(filtered, inc)
	}

	return filtered
}

// MatrixKey returns a stable string key for a matrix combination.
func MatrixKey(combo map[string]string) string {
	keys := make([]string, 0, len(combo))
	for k := range combo {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s=%s", k, combo[k])
	}
	return strings.Join(parts, ",")
}

func cartesian(keys []string, dims map[string][]string) []map[string]string {
	if len(keys) == 0 {
		return []map[string]string{{}}
	}

	key := keys[0]
	rest := cartesian(keys[1:], dims)

	var result []map[string]string
	for _, val := range dims[key] {
		for _, combo := range rest {
			m := make(map[string]string, len(combo)+1)
			for k, v := range combo {
				m[k] = v
			}
			m[key] = val
			result = append(result, m)
		}
	}
	return result
}

func matchesAny(combo map[string]string, excludes []map[string]string) bool {
	for _, exc := range excludes {
		if matchesAll(combo, exc) {
			return true
		}
	}
	return false
}

func matchesAll(combo, pattern map[string]string) bool {
	for k, v := range pattern {
		if combo[k] != v {
			return false
		}
	}
	return true
}

func sortedKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
