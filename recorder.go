package jev

import (
	"encoding/json"
	"os"
	"sync"
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

func (r *FileRecorder) Record(entry Entry) error {
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

	return json.NewEncoder(r.file).Encode(entry)
}

func (r *FileRecorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	if r.file != nil {
		_ = r.file.Sync()
		return r.file.Close()
	}
	return nil
}

var _ Recorder = (*FileRecorder)(nil)
