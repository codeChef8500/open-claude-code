package memory

// Adapter implements engine.MemoryLoader backed by ReadClaudeMd.
// It is wired at SDK construction time to avoid an import cycle
// between the engine and memory packages.
type Adapter struct{}

// NewAdapter creates a memory.Adapter.
func NewAdapter() *Adapter { return &Adapter{} }

// LoadMemory loads and merges all CLAUDE.md files for the given workDir.
func (a *Adapter) LoadMemory(workDir string) (string, error) {
	injection, err := ReadClaudeMd(workDir)
	if err != nil {
		return "", err
	}
	return injection.MergedContent(), nil
}
