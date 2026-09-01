package main

import "strings"

// The released fixture contains one deliberate compiler bug: the default
// normalizer returns the source token unchanged. A generated typed rewrite
// installs the upper-case normalizer for the next generation.
var normalizationMode = "buggy"

func CompileToken(input string) string {
	switch normalizationMode {
	case "upper":
		return strings.ToUpper(input)
	case "lower":
		return strings.ToLower(input)
	default:
		return input
	}
}

func main() {}
