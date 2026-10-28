package jev

type FileRecorderOptions struct {
	Location string
}

type FileRecorder struct {
}

func NewFileRecorder(opts FileRecorderOptions) *FileRecorder {
	return &FileRecorder{}
}

func (r *FileRecorder) Record(userIntent, toolCalls, existingVerdict string, jevProbability, jevLatencyMs float64) error {
	panic("niy")
}
