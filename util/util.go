package util

import (
	"errors"
	"io"
	"io/fs"
	"maps"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/term"
)

func OsIsUnixLike() bool {
	return runtime.GOOS != "windows"
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

func StrArrToRune(sArr []string) [][]rune {
	return MapSlice(sArr, func(s string, _ int) []rune {
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

func GetTruncatedStr(s string) string {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		// Fallback to a default width if terminal size can't be determined
		width = 80
	}

	halfWidth := width / 2

	if len(s) > halfWidth-3 {
		return s[:halfWidth-3] + "..."
	} else {
		return s
	}
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
