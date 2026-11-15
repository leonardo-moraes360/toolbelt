package main

import "fmt"

func main() {
	result := divide(10, 0)
	fmt.Printf("Result: %d\n", result)
}

// divide divides a by b
// BUG: doesn't handle division by zero
func divide(a, b int) int {
	return a / b
}
