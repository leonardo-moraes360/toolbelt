package service

import "localize-callers/mathutil"

func Total(price, qty int) int {
	return mathutil.Scale(price, qty)
}

func Discounted(price, qty, discount int) int {
	base := Total(price, qty)
	return mathutil.Add(base, -discount)
}
