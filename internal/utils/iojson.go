package utils

import (
	"encoding/json"
	"fmt"
	"os"
)

func EnsureDir(dirPath string) error {
	return os.MkdirAll(dirPath, 0o755)
}

func WriteFile(path string, data []byte) error {
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %q: %w", path, err)
	}
	return nil
}

func WritePrettyJSON(path string, raw []byte) error {
	var tmp any
	if err := json.Unmarshal(raw, &tmp); err != nil {
		return fmt.Errorf("validate json for %q: %w", path, err)
	}
	pretty, err := json.MarshalIndent(tmp, "", "  ")
	if err != nil {
		return fmt.Errorf("pretty-print json for %q: %w", path, err)
	}
	return WriteFile(path, pretty)
}

func PrintLine(line string) {
	fmt.Println(line)
}
