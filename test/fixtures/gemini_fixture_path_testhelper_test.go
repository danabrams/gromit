package fixtures_test

import "path/filepath"

func geminiFixturePath(elem ...string) string {
	parts := append([]string{"..", "..", "test", "fixtures", "gemini"}, elem...)
	return filepath.Join(parts...)
}
