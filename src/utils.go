package main

import (
	"math"
)

func Min(x, y int) int {
	if x > y {
		return y
	}
	return x
}

func Max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

func GetLineNoColWidth(ff *FormattedFile) int {
	count := len(ff.lines)
	if count < 1 {
		return 1
	}

	lastLine := ff.lines[count-1]
	maxLineNo := Max(lastLine.aNum, lastLine.bNum)
	return int(math.Log10(float64(maxLineNo)) + 1.0)
}

func DivMod(numerator int, denominator int) (q int, r int) {
    return numerator / denominator, numerator % denominator
}
