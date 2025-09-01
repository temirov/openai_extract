package utils

import (
	"regexp"
	"strings"
)

func CompileUserPattern(user string) (*regexp.Regexp, error) {
	if looksLikeRegex(user) {
		return regexp.Compile(user)
	}
	// plain string => case-insensitive literal
	return regexp.Compile("(?i)" + regexp.QuoteMeta(user))
}

func looksLikeRegex(s string) bool {
	if strings.HasPrefix(s, "(?") {
		return true
	}
	if strings.ContainsAny(s, `[]()|+\^$\\`) {
		return true
	}
	if strings.Contains(s, "?=") || strings.Contains(s, "?<=") || strings.Contains(s, "?!") {
		return true
	}
	return false
}

func BytesToLower(src []byte) []byte {
	dst := make([]byte, len(src))
	for index := 0; index < len(src); index++ {
		current := src[index]
		if current >= 'A' && current <= 'Z' {
			dst[index] = current + 32
		} else {
			dst[index] = current
		}
	}
	return dst
}

func ToLowerTrim(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func StringsJoinComma(items []string) string {
	if len(items) == 0 {
		return ""
	}
	out := items[0]
	for index := 1; index < len(items); index++ {
		out += "," + items[index]
	}
	return out
}
