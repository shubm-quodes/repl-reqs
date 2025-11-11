package util

import (
	"regexp"
	"slices"
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
