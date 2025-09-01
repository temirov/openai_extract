package utils

import (
	"errors"
	"fmt"
	"time"
)

func ExtractCreateTime(record map[string]any) time.Time {
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

func FormatDatestamp(t time.Time) string {
	return t.Format("010206-1504")
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
