package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeminiSummarizerSummarize(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1beta/models/gemini-test:generateContent", r.URL.Path)
		assert.Equal(t, "test-key", r.URL.Query().Get("key"))

		var request geminiGenerateContentRequest
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&request)) {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		if !assert.NotEmpty(t, request.Contents) {
			http.Error(w, "empty contents", http.StatusBadRequest)
			return
		}

		if !assert.NotEmpty(t, request.Contents[0].Parts) {
			http.Error(w, "empty parts", http.StatusBadRequest)
			return
		}

		assert.Contains(t, request.Contents[0].Parts[0].Text, "Текст обновления")

		_, _ = w.Write([]byte(`{
		"candidates": [
			{
				"content": {
					"parts": [
						{"text": "Короткое резюме обновления."}
					]
				}
			}
		]
	}`))
	}))
	defer server.Close()

	summarizer, err := newGeminiSummarizer("test-key", "gemini-test", time.Second, server.Client(), server.URL)
	require.NoError(t, err)

	summary, err := summarizer.Summarize(context.Background(), strings.Repeat("Очень длинный текст. ", 20), 100)

	require.NoError(t, err)
	assert.Equal(t, "Короткое резюме обновления.", summary)
}

func TestGeminiSummarizerTrimsTooLongSummary(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1beta/models/gemini-test:generateContent", r.URL.Path)
		assert.Equal(t, "test-key", r.URL.Query().Get("key"))

		var request geminiGenerateContentRequest
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&request)) {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		if !assert.NotEmpty(t, request.Contents) {
			http.Error(w, "empty contents", http.StatusBadRequest)
			return
		}

		if !assert.NotEmpty(t, request.Contents[0].Parts) {
			http.Error(w, "empty parts", http.StatusBadRequest)
			return
		}

		assert.Contains(t, request.Contents[0].Parts[0].Text, "Текст обновления")

		_, _ = w.Write([]byte(`{
		"candidates": [
			{
				"content": {
					"parts": [
						{"text": "Короткое резюме обновления."}
					]
				}
			}
		]
	}`))
	}))
	defer server.Close()

	summarizer, err := newGeminiSummarizer("test-key", "gemini-test", time.Second, server.Client(), server.URL)
	require.NoError(t, err)

	summary, err := summarizer.Summarize(context.Background(), "long text", 5)

	require.NoError(t, err)
	assert.Equal(t, "Корот...", summary)
}

func TestNewGeminiSummarizerRequiresAPIKey(t *testing.T) {
	t.Parallel()

	_, err := NewGeminiSummarizer("", "gemini-test", time.Second)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "api key")
}
