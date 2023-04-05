package zaploki

import (
	"encoding/json"
	"time"
)

const lokiSinkKey = "loki"

type lokiSink interface {
	Sync() error
	Close() error
	Write(p []byte) (int, error)
}

// type lokiSink struct{}
type sink struct {
	lokiPusher *lokiPusher
}

func newSink(lp *lokiPusher) lokiSink {
	return sink{
		lokiPusher: lp,
	}
}

func (s sink) Sync() error  { return nil }
func (s sink) Close() error { return nil }

func (s sink) Write(p []byte) (int, error) {
	var entry logEntry
	// _, err := marshmallow.Unmarshal(p, &entry)
	err := json.Unmarshal(p, &entry)
	if err != nil {
		return 0, err
	}
	entry.raw = string(p)
	entry.timestamp = time.Now()
	s.lokiPusher.entries <- entry
	return len(p), nil
}