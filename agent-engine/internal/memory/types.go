package memory

import "time"

// ClaudeMdLevel represents where a CLAUDE.md file was found.
type ClaudeMdLevel string

const (
	LevelGlobal  ClaudeMdLevel = "global"  // ~/.claude/CLAUDE.md
	LevelProject ClaudeMdLevel = "project" // <project>/CLAUDE.md
	LevelLocal   ClaudeMdLevel = "local"   // <project>/.claude/CLAUDE.md
)

// ClaudeMdContent holds the parsed content from a single CLAUDE.md file.
type ClaudeMdContent struct {
	Level    ClaudeMdLevel
	FilePath string
	Content  string
	Includes []string // @include directives expanded
}

// ExtractedMemory is a single memory item distilled from session history by the LLM.
type ExtractedMemory struct {
	ID          string    `json:"id"`
	Content     string    `json:"content"`
	SessionID   string    `json:"session_id"`
	ExtractedAt time.Time `json:"extracted_at"`
	Tags        []string  `json:"tags,omitempty"`
}

// MemoryInjection is the combined view of all memories injected into the system prompt.
type MemoryInjection struct {
	GlobalContent  string
	ProjectContent string
	LocalContent   string
	Extracted      []*ExtractedMemory
}
