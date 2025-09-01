package archive

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
)

type conversationRecord map[string]any

func LoadZipFileMap(zipFilePath string) (map[string][]byte, error) {
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

func FindConversationsJSON(fileContentMap map[string][]byte) ([]conversationRecord, error) {
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
