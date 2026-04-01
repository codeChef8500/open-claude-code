package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var fileMentionRe = regexp.MustCompile(`@([\w./\\-]+)`)

// ExpandFileMentions replaces @filepath references in text with the file's
// contents wrapped in XML tags. Non-existent files are left as-is with a note.
func ExpandFileMentions(text, workDir string) string {
	return fileMentionRe.ReplaceAllStringFunc(text, func(match string) string {
		path := match[1:] // strip leading @
		if !filepath.IsAbs(path) {
			path = filepath.Join(workDir, path)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Sprintf("%s [file not found]", match)
		}
		rel, _ := filepath.Rel(workDir, path)
		return fmt.Sprintf("<file path=%q>\n%s\n</file>", rel, strings.TrimRight(string(content), "\n"))
	})
}
