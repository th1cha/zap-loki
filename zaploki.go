package zaploki

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type ZapLoki interface {
	Hook(e zapcore.Entry) error
	Sink(u *url.URL) (zap.Sink, error)
	Stop()
	WithCreateLogger(zap.Config) (*zap.Logger, error)
}

type lokiPusher struct {
	config    *Config
	ctx       context.Context
	cancel    context.CancelFunc
	client    *http.Client
	quit      chan struct{}
	entries   chan logEntry
	waitGroup sync.WaitGroup
}

type lokiPushRequest struct {
	Streams []stream `json:"streams"`
}

type stream struct {
	Stream map[string]string `json:"stream"`
	Values [][2]string       `json:"values"`
}

type logEntry struct {
	Level     string  `json:"level"`
	Timestamp float64 `json:"ts"`
	Message   string  `json:"msg"`
	Caller    string  `json:"caller"`
	raw       string
}

func New(ctx context.Context, cfg Config) ZapLoki {
	cfg.Url = strings.TrimSuffix(cfg.Url, "/")
	cfg.Url = fmt.Sprintf("%s/loki/api/v1/push", cfg.Url)

	ctx, cancel := context.WithCancel(ctx)
	lp := &lokiPusher{
		config:  &cfg,
		ctx:     ctx,
		cancel:  cancel,
		client:  &http.Client{},
		quit:    make(chan struct{}),
		entries: make(chan logEntry, 100), // Adjust buffer size as needed
	}
	lp.waitGroup.Add(1)
	go lp.run()
	return lp
}

// Hook is a function that can be used as a zap hook to write log lines to loki
func (lp *lokiPusher) Hook(e zapcore.Entry) error {
	lp.entries <- logEntry{
		Level:     e.Level.String(),
		Timestamp: float64(e.Time.UnixMilli()),
		Message:   e.Message,
		Caller:    e.Caller.TrimmedPath(),
	}
	return nil
}

// Sink returns a new loki zap sink
func (lp *lokiPusher) Sink(_ *url.URL) (zap.Sink, error) {
	return newSink(lp), nil
}

// Stop stops the loki pusher
func (lp *lokiPusher) Stop() {
	close(lp.quit)
	lp.waitGroup.Wait()
	lp.cancel()
}

var (
	registerSinkOnce sync.Once
)

func registerLokiSink(lp *lokiPusher) error {
	var err error
	registerSinkOnce.Do(func() {
		err = zap.RegisterSink(lokiSinkKey, lp.Sink)
	})
	return err
}

// WithCreateLogger creates a new zap logger with a loki sink from a zap config
func (lp *lokiPusher) WithCreateLogger(cfg zap.Config) (*zap.Logger, error) {
	err := registerLokiSink(lp)
	if err != nil {
		log.Fatal(err)
	}

	fullSinkKey := fmt.Sprintf("%s://", lokiSinkKey)

	if cfg.OutputPaths == nil {
		cfg.OutputPaths = []string{fullSinkKey}
	} else {
		cfg.OutputPaths = append(cfg.OutputPaths, fullSinkKey)
	}

	return cfg.Build()
}

func (lp *lokiPusher) run() {
	ticker := time.NewTicker(lp.config.BatchMaxWait)
	defer ticker.Stop()

	batch := make([]logEntry, 0, lp.config.BatchMaxSize)

	defer func() {
		if len(batch) > 0 {
			lp.send(batch)
		}
		lp.waitGroup.Done()
	}()

	for {
		select {
		case <-lp.ctx.Done():
			return
		case <-lp.quit:
			return
		case entry := <-lp.entries:
			batch = append(batch, entry)
			if len(batch) >= lp.config.BatchMaxSize {
				lp.send(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				lp.send(batch)
				batch = batch[:0]
			}
		}
	}
}

func (lp *lokiPusher) send(batch []logEntry) error {
	data := lokiPushRequest{
		Streams: []stream{
			{
				Stream: lp.config.Labels,
				Values: formatLogEntries(batch),
			},
		},
	}

	msg, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal json: %w", err)
	}

	var buf bytes.Buffer
	g := gzip.NewWriter(&buf)
	if _, err := g.Write(msg); err != nil {
		return fmt.Errorf("failed to gzip json: %w", err)
	}
	if err := g.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}

	req, err := http.NewRequest("POST", lp.config.Url, &buf)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")

	if lp.config.Auth != nil {
		lp.config.Auth.Apply(req)
	}

	resp, err := lp.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("recieved unexpected response code from Loki: %s", resp.Status)
	}

	return nil
}

func formatLogEntries(entries []logEntry) [][2]string {
	logs := make([][2]string, len(entries))
	for i, entry := range entries {
		ts := time.Unix(int64(entry.Timestamp), 0)
		logs[i] = [2]string{strconv.FormatInt(ts.UnixNano(), 10), entry.raw}
	}
	return logs
}
