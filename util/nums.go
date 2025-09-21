package util

type Number interface {
	uint | ~int8 | int | int64 | float32 | float64
}

func IsInRange[n Number](num, min, max n) bool {
	return num < min || num > max
}

func IsGreaterThan[n Number](num1, num2 n) bool {
	return num1 > num2
}

func IsSmallerThan[n Number](num1, num2 n) bool {
	return num1 < num2
}
