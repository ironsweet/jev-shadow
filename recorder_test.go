package jev

import (
	"bufio"
	"encoding/json"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecorder_WritesAllFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jev-records.jsonl")
	r := NewFileRecorder(FileRecorderOptions{Location: path})
	assert.NoError(t, r.Record("intent", "tool calls", "", 0.8849556062190528, 2069.2))
	assert.NoError(t, r.Close())

	// Scan the file and ensure each line is a valid Entry
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	require.NoError(t, err)
	defer f.Close()

	var count int
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry Entry
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &entry))
		assert.Equal(t, "intent", entry.UserIntent)
		assert.Equal(t, "tool calls", entry.ToolCalls)
		assert.Equal(t, "", entry.ExistingVerdict)
		assert.Equal(t, 0.8849556062190528, entry.JEVProbability)
		assert.Equal(t, 2069.2, entry.LatencyMs)
		assert.False(t, entry.Timestamp.IsZero(), "timestamp should be set")
		count++
	}
	require.NoError(t, scanner.Err())
	assert.Equal(t, 1, count, "expected exactly one entry in the file")

}

func TestRecorder_ConcurrentSafe(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jev-concurrent-records.jsonl")
	r := NewFileRecorder(FileRecorderOptions{Location: path})
	var waitGroup sync.WaitGroup
	for range 10 {
		waitGroup.Go(func() {
			for range 10 {
				assert.NoError(t, r.Record("i", "t", "v", rand.Float64(), rand.Float64()*1000))
			}
		})
	}
	waitGroup.Wait()
	assert.NoError(t, r.Close())

	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	require.NoError(t, err)
	defer f.Close()

	var count int
	var scanner = bufio.NewScanner(f)
	for scanner.Scan() {
		var entry Entry
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &entry))
		count++
	}
	require.NoError(t, scanner.Err())
	assert.Equal(t, 100, count, "expected exactly 100 entries in the file")
}

func TestRecorder_WriteErrorReturned(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent-dir", "jev-records.jsonl")
	r := NewFileRecorder(FileRecorderOptions{Location: path})
	err := r.Record("intent", "tool calls", "", 0.5, 100)
	assert.Error(t, err, "expected error when writing to a non-existent directory")
	assert.NoError(t, r.Close())
}

func TestRecorder_PreservesExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jev-cont-records.jsonl")
	require.NoError(t, os.WriteFile(path, []byte(`{"timestamp":"2024-06-01T12:00:00Z","user_intent":"existing intent","tool_calls":"existing tool calls","existing_verdict":"existing verdict","jev_probability":0.5,"latency_ms":100}`+"\n"), 0644))
	r := NewFileRecorder(FileRecorderOptions{Location: path})
	assert.NoError(t, r.Record("intent1", "tool calls1", "", 0.1, 100))
	assert.NoError(t, r.Record("intent2", "tool calls2", "", 0.2, 200))
	assert.NoError(t, r.Close())

	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	require.NoError(t, err)
	defer f.Close()

	var entries []Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry Entry
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &entry))
		entries = append(entries, entry)
	}
	require.NoError(t, scanner.Err())
	assert.Len(t, entries, 3, "expected exactly 3 entries in the file")
	assert.Equal(t, "existing intent", entries[0].UserIntent)
	assert.Equal(t, "intent1", entries[1].UserIntent)
	assert.Equal(t, "intent2", entries[2].UserIntent)
}
