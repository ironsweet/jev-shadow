package jev

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

type FileRecorderOptions struct {
	Location string
}

type FileRecorder struct {
	mu       sync.Mutex
	location string
	file     *os.File
	openErr  error
	closed   bool
}

func NewFileRecorder(opts FileRecorderOptions) *FileRecorder {
	return &FileRecorder{
		location: opts.Location,
	}
}

func (r *FileRecorder) Record(userIntent, toolCalls, existingVerdict string, jevProbability, jevLatencyMs float64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return ErrRecorderClosed
	}
	if r.file == nil && r.openErr == nil {
		f, openErr := os.OpenFile(r.location, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if openErr != nil {
			r.openErr = openErr
			return openErr
		}
		r.file = f
	}
	if r.openErr != nil {
		return r.openErr
	}

	return json.NewEncoder(r.file).Encode(Entry{
		Timestamp:       time.Now(),
		UserIntent:      userIntent,
		ToolCalls:       toolCalls,
		ExistingVerdict: existingVerdict,
		JEVProbability:  jevProbability,
		LatencyMs:       jevLatencyMs,
	})
}

func (r *FileRecorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	if r.file != nil {
		return r.file.Close()
	}
	return nil
}
