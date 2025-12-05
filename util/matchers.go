package util

import (
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

func NavigateToKey(data any, key string) (any, error) {
	switch v := data.(type) {
	case map[string]any:
		if val, ok := v[key]; ok {
			return val, nil
		}
		return nil, fmt.Errorf("key '%s' not found in object", key)

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

		// If key is not an index or wildcard, try to collect the key from all array elements
		// This handles cases like: array is at current level, but we want a property from each element
		var results []any
		for _, item := range v {
			if obj, ok := item.(map[string]any); ok {
				if val, exists := obj[key]; exists {
					results = append(results, val)
				}
			}
		}

		if len(results) == 0 {
			return nil, fmt.Errorf("key '%s' not found in any array element", key)
		}

		// If all results are the same type and simple values, return them
		return results, nil

	default:
		return nil, fmt.Errorf("cannot navigate from type %T with key '%s'", data, key)
	}
}
