package extract

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"

	"openai_extract/internal/archive"
	"openai_extract/internal/filters"
	"openai_extract/internal/utils"

	"go.uber.org/zap"
)

type conversationRecord map[string]any

func Run(archiveFilePath string, searchPatterns []string, outputRoot string, desiredContentTypes []string, desiredLanguages []string) error {
	logger, loggerErr := zap.NewProduction()
	if loggerErr != nil {
		return fmt.Errorf("init logger: %w", loggerErr)
	}
	defer logger.Sync()

	absoluteOutputRoot, absErr := filepath.Abs(outputRoot)
	if absErr != nil {
		return fmt.Errorf("resolve output folder: %w", absErr)
	}
	if mkErr := utils.EnsureDir(absoluteOutputRoot); mkErr != nil {
		return fmt.Errorf("create output folder %q: %w", absoluteOutputRoot, mkErr)
	}

	fileContentMap, loadErr := archive.LoadZipFileMap(archiveFilePath)
	if loadErr != nil {
		return loadErr
	}

	conversations, convoErr := archive.FindConversationsJSON(fileContentMap)
	if convoErr != nil {
		return convoErr
	}

	compiled := make([]*regexp.Regexp, 0, len(searchPatterns))
	for _, patternText := range searchPatterns {
		re, reErr := utils.CompileUserPattern(patternText)
		if reErr != nil {
			return fmt.Errorf("invalid pattern %q: %w", patternText, reErr)
		}
		compiled = append(compiled, re)
	}

	matchedCount := 0
	usedFolderNames := make(map[string]int)

	for _, record := range conversations {
		serialized, serErr := json.Marshal(record)
		if serErr != nil {
			logger.Error("serialize conversation", zap.Error(serErr))
			continue
		}
		lower := utils.BytesToLower(serialized)

		allMatch := true
		for _, re := range compiled {
			if !re.Match(lower) {
				allMatch = false
				break
			}
		}
		if !allMatch {
			continue
		}

		contentTypes := filters.EnumerateContentTypes(serialized)
		if !filters.HasAllDesired(contentTypes, desiredContentTypes, utils.ToLowerTrim) {
			continue
		}

		languages := filters.EnumerateLanguages(serialized)
		if !filters.HasAllDesired(languages, desiredLanguages, filters.NormalizeLanguageName) {
			continue
		}

		startTime := utils.ExtractCreateTime(record)
		baseFolder := utils.FormatDatestamp(startTime)

		if usedFolderNames[baseFolder] > 0 {
			usedFolderNames[baseFolder]++
			baseFolder = fmt.Sprintf("%s_%d", baseFolder, usedFolderNames[baseFolder])
		} else {
			usedFolderNames[baseFolder] = 1
		}

		targetFolder := filepath.Join(absoluteOutputRoot, baseFolder)
		if mkErr := utils.EnsureDir(targetFolder); mkErr != nil {
			logger.Error("create output subfolder", zap.String("folder", targetFolder), zap.Error(mkErr))
			continue
		}

		conversationJSONPath := filepath.Join(targetFolder, "conversation.json")
		if writeErr := utils.WritePrettyJSON(conversationJSONPath, serialized); writeErr != nil {
			logger.Error("write conversation.json", zap.String("path", conversationJSONPath), zap.Error(writeErr))
			continue
		}

		linked := filters.CollectLinkedFiles(serialized, fileContentMap)
		if len(linked) > 0 {
			filesFolder := filepath.Join(targetFolder, "files")
			if mkErr := utils.EnsureDir(filesFolder); mkErr != nil {
				logger.Error("create files subfolder", zap.String("folder", filesFolder), zap.Error(mkErr))
			} else {
				for archivePath, content := range linked {
					targetPath := filepath.Join(filesFolder, filepath.Base(archivePath))
					if writeErr := utils.WriteFile(targetPath, content); writeErr != nil {
						logger.Error("write linked file", zap.String("archivePath", archivePath), zap.String("targetPath", targetPath), zap.Error(writeErr))
					}
				}
			}
		}

		utils.PrintLine(targetFolder + string(filepath.Separator))
		matchedCount++
	}

	if matchedCount == 0 {
		return filters.BuildNoMatchError(utils.StringsJoinComma(searchPatterns), desiredContentTypes, desiredLanguages)
	}
	return nil
}
