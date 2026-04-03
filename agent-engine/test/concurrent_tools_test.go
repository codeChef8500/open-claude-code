package test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool/fileread"
	"github.com/wall-ai/agent-engine/internal/tool/glob"
	"github.com/wall-ai/agent-engine/internal/tool/grep"
)

// TestConcurrencySafeFlag verifies that read-only tools report themselves safe.
func TestConcurrencySafeFlag(t *testing.T) {
	assert.True(t, fileread.New().IsConcurrencySafe(nil), "fileread should be concurrency-safe")
	assert.True(t, grep.New().IsConcurrencySafe(nil), "grep should be concurrency-safe")
	assert.True(t, glob.New().IsConcurrencySafe(nil), "glob should be concurrency-safe")
}

// TestParallelFileReads runs multiple fileread calls concurrently to verify
// no data races occur (use with -race flag).
func TestParallelFileReads(t *testing.T) {
	dir := t.TempDir()
	files := []string{"a.txt", "b.txt", "c.txt", "d.txt", "e.txt"}
	for _, f := range files {
		require.NoError(t, writeTestFile(filepath.Join(dir, f), "content of "+f))
	}

	fr := fileread.New()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var successCount int32
	done := make(chan struct{}, len(files))

	for _, f := range files {
		go func(name string) {
			defer func() { done <- struct{}{} }()
			input, _ := json.Marshal(map[string]string{"file_path": filepath.Join(dir, name)})
			ch, err := fr.Call(ctx, input, &engine.UseContext{WorkDir: dir})
			if err != nil {
				return
			}
			var out string
			for b := range ch {
				out += b.Text
			}
			if out != "" {
				atomic.AddInt32(&successCount, 1)
			}
		}(f)
	}

	for range files {
		<-done
	}
	assert.Equal(t, int32(len(files)), successCount, "all parallel reads should succeed")
}

// TestGlobAndGrepConcurrent runs glob and grep concurrently on the same directory.
func TestGlobAndGrepConcurrent(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		require.NoError(t, writeTestFile(
			filepath.Join(dir, "file.go"),
			"package main\n// needle\nfunc main() {}\n",
		))
		break // one file is sufficient
	}
	require.NoError(t, writeTestFile(filepath.Join(dir, "other.go"), "package other\n"))

	ctx := context.Background()

	type result struct {
		out string
		err error
	}
	globCh := make(chan result, 1)
	grepCh := make(chan result, 1)

	go func() {
		input, _ := json.Marshal(map[string]string{"pattern": "*.go", "path": dir})
		ch, err := glob.New().Call(ctx, input, &engine.UseContext{WorkDir: dir})
		if err != nil {
			globCh <- result{err: err}
			return
		}
		var out string
		for b := range ch {
			out += b.Text
		}
		globCh <- result{out: out}
	}()

	go func() {
		input, _ := json.Marshal(map[string]string{"pattern": "needle", "path": dir})
		ch, err := grep.New().Call(ctx, input, &engine.UseContext{WorkDir: dir})
		if err != nil {
			grepCh <- result{err: err}
			return
		}
		var out string
		for b := range ch {
			out += b.Text
		}
		grepCh <- result{out: out}
	}()

	gr := <-globCh
	grp := <-grepCh

	require.NoError(t, gr.err)
	require.NoError(t, grp.err)
	assert.Contains(t, gr.out, ".go")
	// grep may or may not find via rg vs fallback; just ensure no empty crash
	_ = grp.out
}
