package util

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"maps"
	"os"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/term"
)

const ansi = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"

var re = regexp.MustCompile(ansi)

func OsIsUnixLike() bool {
	return runtime.GOOS != "windows"
}

func ContainsRuneSlice(haystack [][]rune, needle []rune) bool {
	for _, rSlice := range haystack {
		if slices.Equal(rSlice, needle) {
			return true
		}
	}
	return false
}

func IntersectSlice[T comparable](sliceA, sliceB []T) []T {
	aSet := make(map[T]struct{})
	for _, item := range sliceA {
		aSet[item] = struct{}{}
	}

	intersection := make([]T, 0)
	added := make(map[T]struct{})

	for _, item := range sliceB {
		if _, existsInA := aSet[item]; existsInA {
			if _, alreadyAdded := added[item]; !alreadyAdded {
				intersection = append(intersection, item)
				added[item] = struct{}{} // Mark as added
			}
		}
	}

	return intersection
}

func SymmetricDifference[T comparable](sliceA, sliceB []T) []T {
	aSet := make(map[T]struct{})
	bSet := make(map[T]struct{})

	for _, item := range sliceA {
		aSet[item] = struct{}{}
	}
	for _, item := range sliceB {
		bSet[item] = struct{}{}
	}

	symmetricDiff := make([]T, 0)
	added := make(map[T]struct{})

	for _, item := range sliceA {
		if _, existsInB := bSet[item]; !existsInB {
			if _, alreadyAdded := added[item]; !alreadyAdded {
				symmetricDiff = append(symmetricDiff, item)
				added[item] = struct{}{}
			}
		}
	}

	for _, item := range sliceB {
		if _, existsInA := aSet[item]; !existsInA {
			if _, alreadyAdded := added[item]; !alreadyAdded {
				symmetricDiff = append(symmetricDiff, item)
				added[item] = struct{}{}
			}
		}
	}

	return symmetricDiff
}

func RuneArrToStrArr(rSlices [][]rune) []string {
	sSlices := make([]string, len(rSlices))
	for i, rSlice := range rSlices {
		sSlices[i] = string(rSlice)
	}
	return sSlices
}

func TrimRunesLeft(line []rune) []rune {
	var nonSpaceIdx int
	for idx, r := range line {
		if !unicode.IsSpace(r) {
			nonSpaceIdx = idx
			break
		}
	}
	return line[nonSpaceIdx:]
}

func TrimRunesRight(line []rune) []rune {
	var nonSpaceIdx int
	for idx := len(line) - 1; idx > 0; idx-- {
		r := line[idx]
		if !unicode.IsSpace(r) {
			nonSpaceIdx = idx
			break
		}
	}
	return line[:nonSpaceIdx+1]
}

func TrimRunes(line []rune) []rune {
	line = TrimRunesLeft(line)
	line = TrimRunesRight(line)
	return line
}

func FileDoesNotExist(filePath string) bool {
	_, err := os.Stat(filePath)
	return errors.Is(err, fs.ErrNotExist)
}

// Splits the line into tokens, the result can be used for further processing.
func TokenizeRunes(line []rune) [][]rune {
	var tokens [][]rune
	var start int
	inToken := false

	for i, r := range line {
		if unicode.IsSpace(r) {
			if inToken {
				tokens = append(tokens, line[start:i])
				inToken = false
			}
		} else {
			if !inToken {
				start = i
				inToken = true
			}
			// Handle last character case
			if i == len(line)-1 {
				tokens = append(tokens, line[start:i+1])
			}
		}
	}

	return tokens
}

func Split(line []rune) (tokens [][]rune) {
	for idx := 0; idx < len(line); idx++ {
		r := line[idx]
		if unicode.IsSpace(r) {
			tokens = append(tokens, line[0:idx])
			line = TrimRunesLeft(line)
		}
	}
	return
}

func MapSlice[T, M any](s []T, mapFn func(elem T, idx int) M) []M {
	mappedSlice := make([]M, len(s))
	for i, v := range s {
		mappedSlice[i] = mapFn(v, i)
	}
	return mappedSlice
}

func IsEmptyStr(str string) bool {
	return strings.Trim(str, " ") == ""
}

func ArrIncl[E comparable](arr []E, elem E) (includes bool) {
	return slices.Contains(arr, elem)
}

func ArrInclObj(arr []map[string]any, key string) (includes bool) {
	for _, mp := range arr {
		if _, found := mp[key]; found {
			includes = true
			break
		}
	}
	return
}

func StrArrToRune[T ~string](sArr []T) [][]rune {
	return MapSlice(sArr, func(s T, _ int) []rune {
		return []rune(s)
	})
}

/*
*
In case of nil dst maps, a non-nil value will first be initalized before copying
the data from src.
*
*/
func CopyMap[K comparable, V any](dst, src map[K]V) map[K]V {
	if src == nil {
		return dst
	}
	if dst == nil {
		dst = make(map[K]V)
	}
	maps.Copy(dst, src)
	return dst
}

func Is(whatIs string, metadata map[string]any) bool {
	if indeed, ok := metadata[whatIs].(bool); ok && indeed {
		return true
	}
	return false
}

func GetMapVal(keySequence []string, m map[string]any) any {
	for idx, key := range keySequence {
		if value, found := m[key]; found {
			if v, indeed := value.(map[string]any); indeed {
				m = v
			} else if len(keySequence)-1 == idx {
				return value
			} else {
				return nil
			}
		}
	}
	return nil
}

func CompareStrWithAny(val1 string, val2 any) bool {
	switch v := val2.(type) {
	case string:
		return CompareTypeWithAny(val1, v)
	case int:
		intVal, err := strconv.Atoi(val1)
		if err != nil {
			return false
		}
		return CompareTypeWithAny(intVal, v)
	case float64:
		floatVal, err := strconv.ParseFloat(val1, 64)
		if err != nil {
			return false
		}
		return CompareTypeWithAny(floatVal, v)
	case nil:
		if val1 == "nil" {
			return true
		}
		return false
	case []any:
		CheckArrElem(val1, v)
	default:
		return CompareTypeWithAny(v, val2)
	}
	return false
}

func CompareTypeWithAny[T comparable](val1 T, val2 T) bool {
	return val1 == val2
}

func CheckArrElem(elem any, arr []any) bool {
	if len(arr) == 0 {
		return false
	}
	switch arr[0].(type) {
	case map[string]any:
		// return ArrInclObj(valArr, elem)
	default:
		// return ArrIncl(valArr, &elem)
	}
	return false
}

func ReaderToString(reader io.Reader) (string, error) {
	b, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func getTruncatedStrCore(s string, width int) string {
	if width <= 6 {
		return s
	}

	halfWidth := width / 2

	if len(s) > halfWidth-3 {
		return s[:halfWidth-3] + "..."
	}

	return s
}

func GetTruncatedStr(s string) string {
	const defaultFallbackWidth = 120

	width, _, err := term.GetSize(int(os.Stdout.Fd()))

	if err != nil || width == 0 {
		width = defaultFallbackWidth
	}

	return getTruncatedStrCore(s, width)
}

func GetTruncatedStrWithWidth(s string, width int) string {
	if width <= 0 {
		return s
	}

	return getTruncatedStrCore(s, width)
}

func mapSlice[T any, R any](
	slice []T,
	mapFunc func(elem T, idx int) R,
) []R {
	mappedSlice := make([]R, 0)
	for idx, elem := range slice {
		mappedSlice = append(mappedSlice, mapFunc(elem, idx))
	}
	return mappedSlice
}

func AreEmptyStrs(strs ...string) bool {
	for _, s := range strs {
		if strings.Trim(s, " ") == " " {
			return true
		}
	}
	return false
}

func StripAnsi(str string) string {
	return re.ReplaceAllString(str, "")
}

func SliceDiff[T comparable](a, b []T) []T {
	bSet := make(map[T]struct{}, len(b))

	for _, item := range b {
		bSet[item] = struct{}{}
	}

	var result []T
	for _, item := range a {
		if _, found := bSet[item]; !found {
			result = append(result, item)
		}
	}

	return result
}

func RuneSliceDiff(a, b [][]rune) [][]rune {
	bSet := make(map[string]struct{}, len(b))
	for _, runeSlice := range b {
		strKey := string(runeSlice)
		bSet[strKey] = struct{}{}
	}

	var result [][]rune
	for _, runeSlice := range a {
		strKey := string(runeSlice)

		if _, found := bSet[strKey]; !found {
			result = append(result, runeSlice)
		}
	}

	return result
}

func ReverseSlice[T any](s []T) []T {
	reversed := make([]T, len(s))

	copy(reversed, s)

	for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}

	return reversed
}

func FilterPrefixedStrsWithOffset[T ~string](data []T, prefix T, omitExact bool) []T {
	var matches []T
	pStr := string(prefix)

	for _, s := range data {
		sStr := string(s)

		if !strings.HasPrefix(sStr, pStr) {
			continue
		}

		if omitExact && sStr == pStr {
			continue
		}

		matches = append(matches, s[len(prefix):])
	}

	return matches
}

func ReadAndResetIoCloser(ioCloser *io.ReadCloser) ([]byte, error) {
	if ioCloser == nil || *ioCloser == nil {
		return nil, nil
	}

	data, err := io.ReadAll(*ioCloser)
	if err != nil {
		return nil, err
	}

	// Reset the body for next read
	*ioCloser = io.NopCloser(bytes.NewReader(data))

	return data, nil
}
