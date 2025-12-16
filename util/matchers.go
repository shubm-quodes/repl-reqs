package util

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type MatchCriteria[V any] struct {
	M              map[string]V
	Search         string
	IgnorePatterns []string
	Offset         int    // optional
	PrefixWith     string // optional
	SuffixWith     string // optional
	ConverToRune   bool
}

func GetMatchingMapKeysAsRunes[V any](opts *MatchCriteria[V]) [][]rune {
	suggestions, offset := make([][]rune, 0), len(opts.Search)
	isStrEqual := func(s string) bool {
		return strings.HasPrefix(s, opts.Search) &&
			s != strings.TrimRight(opts.Search, "\n")
	}
	for s := range opts.M {
		if isStrEqual(s) && !slices.Contains(opts.IgnorePatterns, s) {
			suggStr := surroundStr(s[offset:], opts.PrefixWith, opts.SuffixWith)
			suggestions = append(suggestions, []rune(suggStr))
		}
	}
	return suggestions
}

func GetMatchingMapKeysAsStr[V any](opts *MatchCriteria[V]) []string {
	suggestions, offset := make([]string, 0), len(opts.Search)
	for s := range opts.M {
		if strings.HasPrefix(s, opts.Search) &&
			s != strings.TrimRight(opts.Search, "\n") {
			suggStr := surroundStr(s[offset:], opts.PrefixWith, opts.SuffixWith)
			suggestions = append(suggestions, suggStr)
		}
	}
	return suggestions
}

func surroundStr(str, prefix, suffix string) string {
	return prefix + str + suffix
}

// Var pattern - `{{(.*?)}}`
func ReplaceStrPattern(input, pattern string, lookups map[string]string) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", err
	}
	matches := re.FindAllStringSubmatch(input, -1)

	if len(matches) == 0 {
		return input, nil
	}

	result := input
	for _, match := range matches {
		varName := strings.TrimSpace(match[1])
		if value, ok := lookups[varName]; ok {
			result = strings.ReplaceAll(result, match[0], value)
		}
	}
	return result, nil
}

// Finds the first match in an array and returns it, else reports with an error
func FindMatchingVal(arr []any, key string) (any, error) {
	for _, item := range arr {
		if obj, ok := item.(map[string]any); ok {
			if val, exists := obj[key]; exists {
				return val, nil
			}
		}
	}

	return nil, fmt.Errorf("none of the array elements contain '%s'", key)
}

// Returns all matching elements in an array, else if no element is found reports with an error
func FindAllMatchingVals(arr []any, key string) ([]any, error) {
	var results []any
	for _, item := range arr {
		if obj, ok := item.(map[string]any); ok {
			if val, exists := obj[key]; exists {
				results = append(results, val)
			}
		}
	}

	return results, fmt.Errorf("none of the array elements contain '%s'", key)
}

func findInMap[V any](data map[string]V, key string) (any, error) {
	if val, ok := data[key]; ok {
		// val (of type V) is implicitly converted to any upon return
		return val, nil
	}
	return nil, fmt.Errorf("key '%s' not found in object", key)
}

func NavigateToKey(data any, key string) (any, error) {
	switch v := data.(type) {
	// 1. The primary, most flexible case (map[string]any)
	case map[string]any:
		return findInMap(v, key)

	// 2. Cases for common, non-any value maps
	// Yeah yeah.. I will NOT use reflection, there's no point in trying to convince me.
	case map[string]bool:
		return findInMap(v, key)
	case map[string]int:
		return findInMap(v, key)
	case map[string]int32:
		return findInMap(v, key)
	case map[string]int64:
		return findInMap(v, key)
	case map[string]float32:
		return findInMap(v, key)
	case map[string]float64:
		return findInMap(v, key)
	case map[string]string:
		return findInMap(v, key)

	// 3. Array/Slice Navigation
	case []any:
		if idx, err := strconv.Atoi(key); err == nil {
			if idx < 0 || idx >= len(v) {
				return nil, fmt.Errorf("array index %d out of range (length: %d)", idx, len(v))
			}
			return v[idx], nil
		}

		// Check if key is wildcard "*"
		if key == "*" {
			return v, nil
		}

		// If key is not an index or wildcard, attempt to find it in array
		return FindMatchingVal(v, key)

	default:
		return nil, fmt.Errorf("cannot navigate from type %T with key '%s'", data, key)
	}
}

// Takes a string pattern to find a value in an array/map[string]any
// In case of arrays first matching val will be returned
func ExtractVal(ds any, pattern string) (any, error) {
	pattern = strings.Trim(pattern, " ")
	if pattern == "" {
		return nil, errors.New("failed to determine val as the pattern is empty")
	}

	var (
		parts []string
		val   any = ds
		err   error
	)

	parts = strings.Split(pattern, ".")
	for _, p := range parts {
		val, err = NavigateToKey(val, p)
		if err != nil {
			break
		}
	}

	return val, err
}
