package main

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type conversationRecord map[string]any

func loadZipFileMap(zipFilePath string) (map[string][]byte, error) {
	zipReader, openErr := zip.OpenReader(zipFilePath)
	if openErr != nil {
		return nil, fmt.Errorf("open zip: %w", openErr)
	}
	defer zipReader.Close()

	fileContentMap := make(map[string][]byte)
	for _, zipFile := range zipReader.File {
		fileReader, openFileErr := zipFile.Open()
		if openFileErr != nil {
			return nil, fmt.Errorf("open zip entry %q: %w", zipFile.Name, openFileErr)
		}
		contentBytes, readErr := io.ReadAll(fileReader)
		fileReader.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read zip entry %q: %w", zipFile.Name, readErr)
		}
		normalizedName := filepath.ToSlash(zipFile.Name)
		fileContentMap[normalizedName] = contentBytes
	}
	return fileContentMap, nil
}

func findConversationsJSON(fileContentMap map[string][]byte) ([]conversationRecord, error) {
	var candidateKeys []string
	for key := range fileContentMap {
		lowerKey := strings.ToLower(key)
		if lowerKey == "conversations.json" || strings.HasSuffix(lowerKey, "/conversations.json") {
			candidateKeys = append(candidateKeys, key)
		}
	}
	sort.Strings(candidateKeys)
	if len(candidateKeys) == 0 {
		return nil, errors.New("conversations.json not found in archive")
	}
	raw := fileContentMap[candidateKeys[0]]
	var records []conversationRecord
	if unmarshalErr := json.Unmarshal(raw, &records); unmarshalErr != nil {
		return nil, fmt.Errorf("parse conversations.json: %w", unmarshalErr)
	}
	return records, nil
}

func bytesToLower(src []byte) []byte {
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

func conversationMatches(record conversationRecord, caseInsensitivePattern *regexp.Regexp) (bool, []byte, error) {
	serialized, marshalErr := json.Marshal(record)
	if marshalErr != nil {
		return false, nil, marshalErr
	}
	if caseInsensitivePattern.Match(bytesToLower(serialized)) {
		return true, serialized, nil
	}
	return false, serialized, nil
}

func parseInt64Strict(s string) (int64, error) {
	var result int64
	if len(s) == 0 {
		return 0, errors.New("empty string")
	}
	for index := 0; index < len(s); index++ {
		ch := s[index]
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("non-digit %q", ch)
		}
		result = result*10 + int64(ch-'0')
	}
	return result, nil
}

func extractCreateTime(record conversationRecord) time.Time {
	candidateKeys := []string{"create_time", "createTime", "create-time", "start_time"}
	for _, key := range candidateKeys {
		rawValue, exists := record[key]
		if !exists {
			continue
		}
		switch typed := rawValue.(type) {
		case float64:
			seconds := int64(typed)
			if seconds > 0 {
				return time.Unix(seconds, 0)
			}
		case string:
			if parsed, err := time.Parse(time.RFC3339, typed); err == nil {
				return parsed
			}
			if seconds, err := parseInt64Strict(typed); err == nil && seconds > 0 {
				return time.Unix(seconds, 0)
			}
		}
	}
	return time.Now()
}

func formatDatestamp(t time.Time) string {
	return t.Format("010206-1504")
}

func ensureDir(dirPath string) error {
	return os.MkdirAll(dirPath, 0o755)
}

func writePrettyJSON(path string, raw []byte) error {
	var tmp any
	if err := json.Unmarshal(raw, &tmp); err != nil {
		return fmt.Errorf("validate json for %q: %w", path, err)
	}
	pretty, err := json.MarshalIndent(tmp, "", "  ")
	if err != nil {
		return fmt.Errorf("pretty-print json for %q: %w", path, err)
	}
	if writeErr := os.WriteFile(path, pretty, 0o644); writeErr != nil {
		return fmt.Errorf("write %q: %w", path, writeErr)
	}
	return nil
}

func collectLinkedFiles(conversationJSON []byte, fileContentMap map[string][]byte) map[string][]byte {
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

var (
	reContentType      = regexp.MustCompile(`"content_type"\s*:\s*"([^"]+)"`)
	reTypeField        = regexp.MustCompile(`"type"\s*:\s*"([^"]+)"`)
	reLanguageField    = regexp.MustCompile(`"language"\s*:\s*"([^"]+)"`)
	reCodeFenceLang    = regexp.MustCompile("```([A-Za-z0-9_+-]+)")
	contentTypeHelpMsg = "Filter: include only conversations that contain at least one of these content types (comma-separated or repeated flag). Example: --content-type code,code_interpreter"
	languageHelpMsg    = "Filter: include only conversations that contain at least one of these languages (comma-separated or repeated flag). Examples: --language python,go  or  -l js -l shell"
)

func enumerateContentTypes(conversationJSON []byte) map[string]struct{} {
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

func normalizeLanguageName(name string) string {
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

func enumerateLanguages(conversationJSON []byte) map[string]struct{} {
	result := make(map[string]struct{})
	for _, m := range reLanguageField.FindAllSubmatch(conversationJSON, -1) {
		if len(m) > 1 {
			result[normalizeLanguageName(string(m[1]))] = struct{}{}
		}
	}
	for _, m := range reCodeFenceLang.FindAllSubmatch(conversationJSON, -1) {
		if len(m) > 1 {
			result[normalizeLanguageName(string(m[1]))] = struct{}{}
		}
	}
	return result
}

func hasAnyDesired(found map[string]struct{}, desired []string, normalizer func(string) string) bool {
	if len(desired) == 0 {
		return true
	}
	for _, d := range desired {
		key := normalizer(d)
		if _, ok := found[key]; ok {
			return true
		}
	}
	return false
}

func normalizeCTShorthand(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if a == "-ct" {
			out = append(out, "--content-type")
			continue
		}
		if strings.HasPrefix(a, "-ct=") {
			out = append(out, "--content-type="+a[len("-ct="):])
			continue
		}
		out = append(out, a)
	}
	return out
}

func main() {
	logger, loggerErr := zap.NewProduction()
	if loggerErr != nil {
		fmt.Fprintln(os.Stderr, "failed to init logger:", loggerErr)
		os.Exit(1)
	}
	defer logger.Sync()

	baseName := filepath.Base(os.Args[0])

	rootCmd := &cobra.Command{
		Use:   baseName + " -f <archive_file.zip> -p <grep_pattern> -o <output_folder> [--content-type code,code_interpreter] [--language python,go]",
		Short: "Extract full conversations from an OpenAI ChatGPT export ZIP by pattern",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			viper.SetEnvPrefix("openai_search")
			viper.AutomaticEnv()

			if viper.GetString("file") == "" {
				return errors.New("missing required flag: -f, --file")
			}
			if viper.GetString("pattern") == "" {
				return errors.New("missing required flag: -p, --pattern")
			}
			if viper.GetString("output") == "" {
				return errors.New("missing required flag: -o, --output")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			archiveFilePath := viper.GetString("file")
			searchPattern := viper.GetString("pattern")
			outputRoot := viper.GetString("output")
			desiredContentTypes := viper.GetStringSlice("content-type")
			desiredLanguagesRaw := viper.GetStringSlice("language")

			desiredLanguages := make([]string, 0, len(desiredLanguagesRaw))
			for _, v := range desiredLanguagesRaw {
				for _, piece := range strings.Split(v, ",") {
					trimmed := strings.TrimSpace(piece)
					if trimmed != "" {
						desiredLanguages = append(desiredLanguages, trimmed)
					}
				}
			}

			absoluteOutputRoot, absErr := filepath.Abs(outputRoot)
			if absErr != nil {
				return fmt.Errorf("resolve output folder: %w", absErr)
			}
			if mkErr := ensureDir(absoluteOutputRoot); mkErr != nil {
				return fmt.Errorf("create output folder %q: %w", absoluteOutputRoot, mkErr)
			}

			fileContentMap, loadErr := loadZipFileMap(archiveFilePath)
			if loadErr != nil {
				return loadErr
			}

			conversations, convoErr := findConversationsJSON(fileContentMap)
			if convoErr != nil {
				return convoErr
			}

			caseInsensitivePattern, reErr := regexp.Compile("(?i)" + regexp.QuoteMeta(searchPattern))
			if reErr != nil {
				return fmt.Errorf("invalid pattern: %w", reErr)
			}

			matchedCount := 0
			usedFolderNames := make(map[string]int)

			for _, record := range conversations {
				isMatch, serialized, serErr := conversationMatches(record, caseInsensitivePattern)
				if serErr != nil {
					logger.Error("serialize conversation", zap.Error(serErr))
					continue
				}
				if !isMatch {
					continue
				}

				contentTypes := enumerateContentTypes(serialized)
				if !hasAnyDesired(contentTypes, desiredContentTypes, func(s string) string { return strings.ToLower(strings.TrimSpace(s)) }) {
					continue
				}

				langs := enumerateLanguages(serialized)
				if !hasAnyDesired(langs, desiredLanguages, normalizeLanguageName) {
					continue
				}

				startTime := extractCreateTime(record)
				baseFolder := formatDatestamp(startTime)

				if usedFolderNames[baseFolder] > 0 {
					usedFolderNames[baseFolder]++
					baseFolder = fmt.Sprintf("%s_%d", baseFolder, usedFolderNames[baseFolder])
				} else {
					usedFolderNames[baseFolder] = 1
				}

				targetFolder := filepath.Join(absoluteOutputRoot, baseFolder)
				if mkErr := ensureDir(targetFolder); mkErr != nil {
					logger.Error("create output subfolder", zap.String("folder", targetFolder), zap.Error(mkErr))
					continue
				}

				conversationJSONPath := filepath.Join(targetFolder, "conversation.json")
				if writeErr := writePrettyJSON(conversationJSONPath, serialized); writeErr != nil {
					logger.Error("write conversation.json", zap.String("path", conversationJSONPath), zap.Error(writeErr))
					continue
				}

				linked := collectLinkedFiles(serialized, fileContentMap)
				if len(linked) > 0 {
					filesFolder := filepath.Join(targetFolder, "files")
					if mkErr := ensureDir(filesFolder); mkErr != nil {
						logger.Error("create files subfolder", zap.String("folder", filesFolder), zap.Error(mkErr))
					} else {
						for archivePath, content := range linked {
							targetPath := filepath.Join(filesFolder, filepath.Base(archivePath))
							if writeErr := os.WriteFile(targetPath, content, 0o644); writeErr != nil {
								logger.Error("write linked file", zap.String("archivePath", archivePath), zap.String("targetPath", targetPath), zap.Error(writeErr))
							}
						}
					}
				}

				fmt.Println(targetFolder + string(os.PathSeparator))
				matchedCount++
			}

			if matchedCount == 0 {
				ct := strings.Join(viper.GetStringSlice("content-type"), ",")
				lang := strings.Join(viper.GetStringSlice("language"), ",")
				switch {
				case ct != "" && lang != "":
					return fmt.Errorf("no conversations matched pattern %q with content type(s) %q and language(s) %q", searchPattern, ct, lang)
				case ct != "":
					return fmt.Errorf("no conversations matched pattern %q with content type(s) %q", searchPattern, ct)
				case lang != "":
					return fmt.Errorf("no conversations matched pattern %q with language(s) %q", searchPattern, lang)
				default:
					return fmt.Errorf("no conversations matched pattern %q", searchPattern)
				}
			}
			return nil
		},
	}

	rootCmd.Flags().StringP("file", "f", "", "Path to the OpenAI ChatGPT ZIP archive (required)")
	rootCmd.Flags().StringP("pattern", "p", "", "Case-insensitive search pattern (required)")
	rootCmd.Flags().StringP("output", "o", "", "Output folder (required)")
	rootCmd.Flags().StringSlice("content-type", nil, contentTypeHelpMsg)
	rootCmd.Flags().StringSliceP("language", "l", nil, languageHelpMsg)

	_ = viper.BindPFlag("file", rootCmd.Flags().Lookup("file"))
	_ = viper.BindPFlag("pattern", rootCmd.Flags().Lookup("pattern"))
	_ = viper.BindPFlag("output", rootCmd.Flags().Lookup("output"))
	_ = viper.BindPFlag("content-type", rootCmd.Flags().Lookup("content-type"))
	_ = viper.BindPFlag("language", rootCmd.Flags().Lookup("language"))

	normalized := normalizeCTShorthand(os.Args[1:])
	rootCmd.SetArgs(normalized)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
