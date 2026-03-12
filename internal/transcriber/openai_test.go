package transcriber

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAITranscribe(t *testing.T) {
	var receivedModel string
	var receivedFormat string
	var receivedFileData []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/transcriptions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		err := r.ParseMultipartForm(10 << 20) //nolint:gosec // test code with bounded input
		if err != nil {
			t.Errorf("parse multipart: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		receivedModel = r.FormValue("model")            //nolint:gosec // test code
		receivedFormat = r.FormValue("response_format") //nolint:gosec // test code

		file, _, err := r.FormFile("file")
		if err != nil {
			t.Errorf("get form file: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		defer func() { _ = file.Close() }()
		receivedFileData, _ = io.ReadAll(file)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("  Hello world  "))
	}))
	defer server.Close()

	transcriber := NewOpenAI(server.URL, "test-model", 30, false, nil)
	wavData := []byte("fake-wav-data")
	result, err := transcriber.Transcribe(context.Background(), wavData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "Hello world" {
		t.Errorf("expected 'Hello world', got %q", result)
	}
	if receivedModel != "test-model" {
		t.Errorf("expected model 'test-model', got %q", receivedModel)
	}
	if receivedFormat != "text" {
		t.Errorf("expected format 'text', got %q", receivedFormat)
	}
	if string(receivedFileData) != "fake-wav-data" {
		t.Errorf("expected file data 'fake-wav-data', got %q", string(receivedFileData))
	}
}

func TestOpenAITranscribeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model not found", http.StatusNotFound)
	}))
	defer server.Close()

	transcriber := NewOpenAI(server.URL, "bad-model", 30, false, nil)
	_, err := transcriber.Transcribe(context.Background(), []byte("data"))
	if err == nil {
		t.Error("expected error for 404 response")
	}
}
