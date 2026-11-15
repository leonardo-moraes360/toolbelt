package strings

import "strings"

// Reverse reverses a string
func Reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// IsPalindrome checks if a string is a palindrome
func IsPalindrome(s string) bool {
	s = strings.ToLower(s)
	clean := ""
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			clean += string(r)
		}
	}
	return clean == Reverse(clean)
}

// CountWords counts words in a string
func CountWords(s string) int {
	return len(strings.Fields(s))
}

// Truncate truncates a string to maxLen, adding "..." if truncated
func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return "..."[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
