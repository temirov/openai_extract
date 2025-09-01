package filters

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	reContentType   = regexp.MustCompile(`"content_type"\s*:\s*"([^"]+)"`)
	reTypeField     = regexp.MustCompile(`"type"\s*:\s*"([^"]+)"`)
	reLanguageField = regexp.MustCompile(`"language"\s*:\s*"([^"]+)"`)
	reCodeFenceLang = regexp.MustCompile("```([A-Za-z0-9_+-]+)")
)

// EnumerateContentTypes extracts content types present in a conversation JSON blob.
func EnumerateContentTypes(conversationJSON []byte) map[string]struct{} {
	result := make(map[string]struct{})
	for _, m := range reContentType.FindAllSubmatch(conversationJSON, -1) {
		if len(m) > 1 {
			result[strings.ToLower(string(m[1]))] = struct{}{}
		}
	}
	for _, m := range reTypeField.FindAllSubmatch(conversationJSON, -1) {
		if len(m) > 1 {
			result[strings.ToLower(string(m[1]))] = struct{}{}
		}
	}
	return result
}

// NormalizeLanguageName canonicalizes language names.
func NormalizeLanguageName(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	switch n {
	case "js", "node", "nodejs", "javascript":
		return "javascript"
	case "golang":
		return "go"
	case "sh", "bash", "zsh", "shell":
		return "shell"
	case "py", "python3":
		return "python"
	case "ts", "typescript":
		return "typescript"
	case "c++", "cpp":
		return "cpp"
	case "c#":
		return "csharp"
	default:
		return n
	}
}

// EnumerateLanguages extracts languages from JSON "language" fields and Markdown code fences.
func EnumerateLanguages(conversationJSON []byte) map[string]struct{} {
	result := make(map[string]struct{})
	for _, m := range reLanguageField.FindAllSubmatch(conversationJSON, -1) {
		if len(m) > 1 {
			result[NormalizeLanguageName(string(m[1]))] = struct{}{}
		}
	}
	for _, m := range reCodeFenceLang.FindAllSubmatch(conversationJSON, -1) {
		if len(m) > 1 {
			result[NormalizeLanguageName(string(m[1]))] = struct{}{}
		}
	}
	return result
}

// HasAnyDesired returns true if any desired key is present in the found set.
func HasAnyDesired(found map[string]struct{}, desired []string, normalizer func(string) string) bool {
	if len(desired) == 0 {
		return true
	}
	for _, value := range desired {
		key := normalizer(value)
		if _, ok := found[key]; ok {
			return true
		}
	}
	return false
}

// CollectLinkedFiles finds attachments under "files/" referenced by filename in the conversation JSON.
func CollectLinkedFiles(conversationJSON []byte, fileContentMap map[string][]byte) map[string][]byte {
	found := make(map[string][]byte)

	var archiveFiles []string
	for key := range fileContentMap {
		lower := strings.ToLower(filepath.ToSlash(key))
		if strings.HasPrefix(lower, "files/") && !strings.HasSuffix(lower, "/") {
			archiveFiles = append(archiveFiles, key)
		}
	}

	conversationStringLower := strings.ToLower(string(conversationJSON))
	for _, archivePath := range archiveFiles {
		base := strings.ToLower(filepath.Base(archivePath))
		if base == "" {
			continue
		}
		if strings.Contains(conversationStringLower, base) {
			found[archivePath] = fileContentMap[archivePath]
		}
	}
	return found
}

// BuildNoMatchError creates a precise error when nothing matched.
func BuildNoMatchError(patternCSV string, contentTypes []string, languages []string) error {
	ct := strings.Join(contentTypes, ",")
	lang := strings.Join(languages, ",")
	switch {
	case ct != "" && lang != "":
		return fmt.Errorf("no conversations matched patterns [%s] with content type(s) %q and language(s) %q", patternCSV, ct, lang)
	case ct != "":
		return fmt.Errorf("no conversations matched patterns [%s] with content type(s) %q", patternCSV, ct)
	case lang != "":
		return fmt.Errorf("no conversations matched patterns [%s] with language(s) %q", patternCSV, lang)
	default:
		return fmt.Errorf("no conversations matched patterns [%s]", patternCSV)
	}
}

func HasAllDesired(found map[string]struct{}, desired []string, normalizer func(string) string) bool {
	if len(desired) == 0 {
		return true
	}
	for _, value := range desired {
		if _, ok := found[normalizer(value)]; !ok {
			return false
		}
	}
	return true
}
