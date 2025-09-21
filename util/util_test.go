package util

import (
	"reflect"
	"testing"
)

// func TestSplit(t *testing.T) {
// 	str := "set var num"
// 	got := convertToRune(strings.Split(str, " "))
// 	want := convertToRune(strings.Split(str, " "))
// 	flag := false
// 	for i := range want {
// 		if string(want[i]) != string(got[i]) {
// 			flag = true
// 			break
// 		}
// 	}
// 	if flag {
// 		t.Errorf("got %q, wanted %q", got, want)
// 	}
// }

func TestTokenizeRunes(t *testing.T) {
	str := "set var num"
	got := TokenizeRunes([]rune(str))
	want := convertToRune([]string{"set", "var", "num"})
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, wanted %q", got, want)
	}
}

func convertToRune(line []string) (arr [][]rune) {
	for _, str := range line {
		arr = append(arr, []rune(str))
	}

	return
}
