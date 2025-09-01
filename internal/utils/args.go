package utils

import "strings"

func NormalizeCTShorthand(args []string) []string {
	output := make([]string, 0, len(args))
	for _, original := range args {
		if original == "-ct" {
			output = append(output, "--content-type")
			continue
		}
		if strings.HasPrefix(original, "-ct=") {
			output = append(output, "--content-type="+original[len("-ct="):])
			continue
		}
		output = append(output, original)
	}
	return output
}
