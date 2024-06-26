package zaploki

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"go.uber.org/zap"
)

func mockRequestTestServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		if r.Header.Get("Content-Encoding") != "gzip" {
			t.Errorf("Expected Content-Encoding gzip, got %s", r.Header.Get("Content-Encoding"))
		}

		var buf bytes.Buffer
		gzipReader, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Fatalf("Failed to create gzip reader: %v", err)
		}
		defer gzipReader.Close()
		_, err = buf.ReadFrom(gzipReader)
		if err != nil {
			t.Fatalf("Failed to read gzipped data: %v", err)
		}

		var req lokiPushRequest
		err = json.Unmarshal(buf.Bytes(), &req)
		if err != nil {
			t.Fatalf("Failed to unmarshal JSON: %v", err)
		}

		if len(req.Streams) == 0 {
			t.Errorf("Expected at least one stream, got %d", len(req.Streams))
		}

		expectedLabels := map[string]string{"app": "test", "env": "dev", "instance": "instance-1"}
		found := false
		for _, s := range req.Streams {
			if reflect.DeepEqual(s.Stream, expectedLabels) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected labels %v, but did not find them in the request", expectedLabels)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
}

func TestNew(t *testing.T) {
	mockServer := mockRequestTestServer(t)
	defer mockServer.Close()

	v := New(context.Background(), Config{
		Url:          mockServer.URL,
		BatchMaxSize: 100,
		BatchMaxWait: 10 * time.Second,
		Labels:       map[string]string{"app": "test", "env": "dev", "instance": "instance-1"},
	})
	logger, err := v.WithCreateLogger(zap.NewProductionConfig())
	if err != nil {
		t.Fatal(err)
	}

	logger.Info("failed to fetch URL",
		zap.String("url", "https://test.com"),
		zap.Int("attempt", 3),
		zap.Duration("backoff", time.Second),
	)
	defer logger.Sync()
	v.Stop()
}

func TestBasicAuth(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:password"))
		if r.Header.Get("Authorization") != expectedAuth {
			t.Errorf("Expected Authorization header %s, got %s", expectedAuth, r.Header.Get("Authorization"))
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer mockServer.Close()

	v := New(context.Background(), Config{
		Url:          mockServer.URL,
		BatchMaxSize: 100,
		BatchMaxWait: 10 * time.Second,
		Labels:       map[string]string{"app": "test", "env": "dev", "instance": "instance-1"},
		Auth:         &BasicAuthenticator{"user", "password"},
	})
	logger, err := v.WithCreateLogger(zap.NewProductionConfig())
	if err != nil {
		t.Fatal(err)
	}

	logger.Info("failed to fetch URL",
		zap.String("url", "https://test.com"),
		zap.Int("attempt", 3),
		zap.Duration("backoff", time.Second),
	)
	defer logger.Sync()
	v.Stop()
}

func TestApiKeyAuth(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "my-api-key" {
			t.Errorf("Expected 'X-API-Key header 'my-api-key', got '%s'", r.Header.Get("X-API-Key"))
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer mockServer.Close()

	v := New(context.Background(), Config{
		Url:          mockServer.URL,
		BatchMaxSize: 100,
		BatchMaxWait: 10 * time.Second,
		Labels:       map[string]string{"app": "test", "env": "dev", "instance": "instance-1"},
		Auth:         &APIKeyAuthenticator{"X-API-Key", "my-api-key"},
	})
	logger, err := v.WithCreateLogger(zap.NewProductionConfig())
	if err != nil {
		t.Fatal(err)
	}

	logger.Info("failed to fetch URL",
		zap.String("url", "https://test.com"),
		zap.Int("attempt", 3),
		zap.Duration("backoff", time.Second),
	)
	defer logger.Sync()
	v.Stop()
}
