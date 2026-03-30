package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/testcontainers/testcontainers-go"
)

type LinkItem struct {
	ID      int32    `json:"id"`
	URL     string   `json:"url"`
	Tags    []string `json:"tags"`
	Filters []string `json:"filters"`
}
type e2eEnv struct {
	BotBaseURL      string
	ScrapperBaseURL string

	network     *testcontainers.DockerNetwork
	bot         testcontainers.Container
	scrapper    testcontainers.Container
	http        *http.Client
	projectRoot string
}

func (e *e2eEnv) Close(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	if e.bot != nil {
		_ = e.bot.Terminate(ctx)
	}
	if e.scrapper != nil {
		_ = e.scrapper.Terminate(ctx)
	}
	if e.network != nil {
		_ = e.network.Remove(ctx)
	}
}

func (e *e2eEnv) doNoBody(method, url string, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return e.http.Do(req)
}

func (e *e2eEnv) doJSON(method, url string, headers map[string]string, body any) (*http.Response, error) {
	var buf *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		buf = bytes.NewReader(b)
	} else {
		buf = bytes.NewReader(nil)
	}

	req, err := http.NewRequest(method, url, buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return e.http.Do(req)
}

func mustDecodeJSON(t *testing.T, resp *http.Response, out any) {
	t.Helper()
	dec := json.NewDecoder(resp.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		t.Fatalf("decode json: %v", err)
	}
}

func containsURL(items []LinkItem, target string) bool {
	for _, it := range items {
		if it.URL == target {
			return true
		}
	}
	return false
}

func mustNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func mustFindProjectRoot(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	mustNoErr(t, err)

	for dir := wd; dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		if fileExists(filepath.Join(dir, "Dockerfile.scrapper")) && fileExists(filepath.Join(dir, "Dockerfile.bot")) {
			return dir
		}
		if fileExists(filepath.Join(dir, "go.mod")) {
			if fileExists(filepath.Join(dir, "Dockerfile.scrapper")) || fileExists(filepath.Join(dir, "Dockerfile.bot")) {
				return dir
			}
		}
	}
	t.Fatalf("project root not found from wd=%s (expected Dockerfile.scrapper/Dockerfile.bot in repo root)", wd)
	return ""
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func scrOrLocalhost(host string) string {
	if host == "" {
		return "localhost"
	}
	if net.ParseIP(host) != nil {
		return host
	}
	return host
}

func (e *e2eEnv) DumpLogs(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	dumpContainerLogs(ctx, t, e.scrapper, "scrapper")
	dumpContainerLogs(ctx, t, e.bot, "bot")
}

func dumpContainerLogs(ctx context.Context, t *testing.T, c testcontainers.Container, name string) {
	t.Helper()
	if c == nil {
		return
	}
	r, err := c.Logs(ctx)
	if err != nil {
		t.Logf("[%s] cannot read logs: %v", name, err)
		return
	}
	defer r.Close()
	b, _ := io.ReadAll(r)
	t.Logf("=== %s logs ===\n%s\n=== end %s logs ===", name, string(b), name)
}
