package jev

type Judger interface {
	Judge(userIntent, toolCalls, existingVerdict string) (float64, error)
}

type Recorder interface {
	Record(userIntent, toolCalls, existingVerdict string, jevProbability, jevLatencyMs float64) error
	Close() error
}

type Shadow struct {
	judger   Judger
	recorder Recorder
	gate     *Gate
}

func NewShadow(judger Judger, recorder Recorder, gate *Gate) *Shadow {
	return &Shadow{
		judger:   judger,
		recorder: recorder,
		gate:     gate,
	}
}

func (s *Shadow) Submit(userIntent, toolCalls, existingVerdict string, jevProbability, jevLatencyMs float64) error {
	panic("niy")
}
