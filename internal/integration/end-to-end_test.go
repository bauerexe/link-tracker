package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	tcnetwork "github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
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

func TestEndToEnd(t *testing.T) {
	env := mustStartE2EEnv(t)
	defer env.Close(t)

	t.Run("Test1_Bot_CorrectUpdate_2000", func(t *testing.T) {
		body := map[string]any{
			"id":          int64(1),
			"url":         "https://github.com/example/repo",
			"description": "desc",
			"tgChatIds":   []int64{1},
		}

		resp, err := env.doJSON("POST", env.BotBaseURL+"/updates", nil, body)
		if err != nil {
			env.DumpLogs(t)
			t.Fatal(err)
		}
		mustNoErr(t, err)
		defer func(Body io.ReadCloser) {
			_ = Body.Close()
		}(resp.Body)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
		}
	})

	t.Run("Test2_Bot_IncorrectUpdate_Not200", func(t *testing.T) {
		body := map[string]any{
			"id":        "oops",
			"url":       "not-a-uri",
			"tgChatIds": "also-wrong-type",
		}

		resp, err := env.doJSON("POST", env.BotBaseURL+"/updates", nil, body)
		mustNoErr(t, err)
		defer func(Body io.ReadCloser) {
			_ = Body.Close()
		}(resp.Body)

		if resp.StatusCode == http.StatusOK {
			t.Fatalf("expected non-200, got 200")
		}
	})

	t.Run("Test3_1_Scrapper_AddAndGetLink", func(t *testing.T) {
		chatID := int64(101)

		resp, err := env.doNoBody("POST", fmt.Sprintf("%s/tg-chat/%d", env.ScrapperBaseURL, chatID), nil)
		mustNoErr(t, err)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		link := "https://stackoverflow.com/questions/1"
		addReq := map[string]any{
			"link":    link,
			"tags":    []string{"tag1"},
			"filters": []string{"filter1"},
		}

		resp, err = env.doJSON("POST", env.ScrapperBaseURL+"/links", map[string]string{
			"Tg-Chat-Id": fmt.Sprintf("%d", chatID),
		}, addReq)
		mustNoErr(t, err)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var list struct {
			Links []LinkItem `json:"links"`
			Size  int        `json:"size"`
		}
		resp, err = env.doJSON("GET", env.ScrapperBaseURL+"/links", map[string]string{
			"Tg-Chat-Id": fmt.Sprintf("%d", chatID),
		}, nil)
		mustNoErr(t, err)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		mustDecodeJSON(t, resp, &list)

		if !containsURL(list.Links, link) {
			t.Fatalf("expected link %q to still exist, got %+v", link, list.Links)
		}
	})

	t.Run("Test3_2_Scrapper_AddThenDeleteLink", func(t *testing.T) {
		chatID := int64(102)
		link := "https://github.com/golang/go"

		resp, err := env.doNoBody("POST", fmt.Sprintf("%s/tg-chat/%d", env.ScrapperBaseURL, chatID), nil)
		mustNoErr(t, err)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		resp, err = env.doJSON("POST", env.ScrapperBaseURL+"/links", map[string]string{
			"Tg-Chat-Id": fmt.Sprintf("%d", chatID),
		}, map[string]any{
			"chatId": chatID,
			"link":   link,
		})
		mustNoErr(t, err)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		resp, err = env.doJSON("DELETE", env.ScrapperBaseURL+"/links", map[string]string{
			"Tg-Chat-Id": fmt.Sprintf("%d", chatID),
		}, map[string]any{
			"chat_id": chatID,
			"link":    link,
		})
		mustNoErr(t, err)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d, body=%s", resp.StatusCode, string(b))
		}

		var list struct {
			Links []LinkItem `json:"links"`
			Size  int        `json:"size"`
		}
		resp, err = env.doJSON("GET", env.ScrapperBaseURL+"/links", map[string]string{
			"Tg-Chat-Id": fmt.Sprintf("%d", chatID),
		}, nil)
		mustNoErr(t, err)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("GET /links expected 200, got %d, body=%s", resp.StatusCode, string(b))
		}
		mustDecodeJSON(t, resp, &list)

		if containsURL(list.Links, link) {
			t.Fatalf("expected link %q to be removed, got %+v", link, list.Links)
		}
	})

	t.Run("Test3_3_Scrapper_DeleteFromNonexistentChat_Not200_AndLinkStillThere", func(t *testing.T) {
		chatID := int64(103)
		link := "https://example.com"

		resp, err := env.doNoBody("POST", fmt.Sprintf("%s/tg-chat/%d", env.ScrapperBaseURL, chatID), nil)
		mustNoErr(t, err)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		resp, err = env.doJSON("POST", env.ScrapperBaseURL+"/links", map[string]string{
			"Tg-Chat-Id": fmt.Sprintf("%d", chatID),
		}, map[string]any{"link": link})
		mustNoErr(t, err)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		resp, err = env.doJSON("DELETE", env.ScrapperBaseURL+"/links", map[string]string{
			"Tg-Chat-Id": "999",
		}, map[string]any{"link": link})
		mustNoErr(t, err)
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			t.Fatalf("expected non-200, got 200")
		}

		var list struct {
			Links []LinkItem `json:"links"`
			Size  int        `json:"size"`
		}
		resp, err = env.doJSON("GET", env.ScrapperBaseURL+"/links", map[string]string{
			"Tg-Chat-Id": fmt.Sprintf("%d", chatID),
		}, nil)
		mustNoErr(t, err)
		defer func() {
			_ = resp.Body.Close()
		}()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		mustDecodeJSON(t, resp, &list)

		if !containsURL(list.Links, link) {
			t.Fatalf("expected link %q to still exist, got %+v", link, list.Links)
		}
	})

	t.Run("Test3_4_Scrapper_AddLinkToNonexistentChat_Not200", func(t *testing.T) {
		chatID := int64(104)

		resp, err := env.doNoBody("POST", fmt.Sprintf("%s/tg-chat/%d", env.ScrapperBaseURL, chatID), nil)
		mustNoErr(t, err)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		resp, err = env.doJSON("POST", env.ScrapperBaseURL+"/links", map[string]string{
			"Tg-Chat-Id": "2",
		}, map[string]any{"link": "https://example.org"})
		mustNoErr(t, err)
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			t.Fatalf("expected non-200, got 200")
		}
	})

	t.Run("Test3_5_Scrapper_DeletedChat_CannotAddLink", func(t *testing.T) {
		chatID := int64(105)

		resp, err := env.doNoBody("POST", fmt.Sprintf("%s/tg-chat/%d", env.ScrapperBaseURL, chatID), nil)
		mustNoErr(t, err)
		_ = resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		resp, err = env.doNoBody("DELETE", fmt.Sprintf("%s/tg-chat/%d", env.ScrapperBaseURL, chatID), nil)
		mustNoErr(t, err)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		resp, err = env.doJSON("POST", env.ScrapperBaseURL+"/links", map[string]string{
			"Tg-Chat-Id": fmt.Sprintf("%d", chatID),
		}, map[string]any{"link": "https://example.net"})
		mustNoErr(t, err)
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			t.Fatalf("expected non-200, got 200")
		}
	})

	t.Run("Test3_6_Scrapper_DeleteNonexistentChat_404", func(t *testing.T) {
		chatID := int64(999999)
		resp, err := env.doNoBody("DELETE", fmt.Sprintf("%s/tg-chat/%d", env.ScrapperBaseURL, chatID), nil)
		mustNoErr(t, err)
		_ = resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", resp.StatusCode)
		}
	})
}

func mustStartE2EEnv(t *testing.T) *e2eEnv {
	t.Helper()

	root := mustFindProjectRoot(t)
	ctx := context.Background()

	httpClient := &http.Client{Timeout: 10 * time.Second}

	network, err := tcnetwork.New(ctx)
	mustNoErr(t, err)
	networkName := network.Name

	envPath := filepath.Join(root, "app.env")
	commonFiles := []testcontainers.ContainerFile{
		{
			HostFilePath:      envPath,
			ContainerFilePath: "/app/app.env",
			FileMode:          0o644,
		},
	}

	scrReq := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    root,
			Dockerfile: "Dockerfile.scrapper",
		},
		Files:        commonFiles,
		ExposedPorts: []string{"8080/tcp", "50051/tcp"},
		Networks:     []string{networkName},
		NetworkAliases: map[string][]string{
			networkName: {"scrapper"},
		},
		ConfigModifier: func(cfg *dockercontainer.Config) {
			cfg.WorkingDir = "/app"
		},
		WaitingFor: wait.ForLog("gateway listening at port").WithStartupTimeout(10 * time.Second),
		Env: map[string]string{
			"SCRAPPER_ADDR_HTTP":     "0.0.0.0:8080",
			"SCRAPPER_ADDR_GRPC":     "0.0.0.0:50051",
			"BOT_ADDR_GRPC":          "bot:50052",
			"MINUTES_INTERVAL_CHECK": "1",
			"GITHUB_TOKEN":           "",
			"STACK_OVERFLOW_KEY":     "",
		},
	}

	scr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: scrReq,
		Started:          true,
	})
	mustNoErr(t, err)

	botReq := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    root,
			Dockerfile: "Dockerfile.bot",
		},
		Files:        commonFiles,
		ExposedPorts: []string{"8082/tcp", "50052/tcp"},
		Networks:     []string{networkName},
		NetworkAliases: map[string][]string{
			networkName: {"bot"},
		},
		ConfigModifier: func(cfg *dockercontainer.Config) {
			cfg.WorkingDir = "/app"
		},
		WaitingFor: wait.ForLog(`"msg":"starting bot"`).WithStartupTimeout(10 * time.Second),
		Env: map[string]string{
			"BOT_ADDR_HTTP":        "0.0.0.0:8082",
			"BOT_ADDR_GRPC":        "0.0.0.0:50052",
			"SCRAPPER_ADDR_GRPC":   "scrapper:50051",
			"APP_TELEGRAM_TOKEN":   "dummy",
			"BOT_DISABLE_TELEGRAM": "1",
		},
	}

	bot, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: botReq,
		Started:          false,
	})
	mustNoErr(t, err)

	if err = bot.Start(ctx); err != nil {
		dumpContainerLogs(ctx, t, scr, "scrapper")
		dumpContainerLogs(ctx, t, bot, "bot")
		_ = bot.Terminate(ctx)
		_ = scr.Terminate(ctx)
		_ = network.Remove(ctx)
		t.Fatalf("start bot container: %v", err)
	}

	scrHost, err := scr.Host(ctx)
	mustNoErr(t, err)
	scrPort, err := scr.MappedPort(ctx, "8080/tcp")
	mustNoErr(t, err)

	botHost, err := bot.Host(ctx)
	mustNoErr(t, err)
	botPort, err := bot.MappedPort(ctx, "8082/tcp")
	mustNoErr(t, err)

	return &e2eEnv{
		BotBaseURL:      fmt.Sprintf("http://%s:%s", scrOrLocalhost(botHost), botPort.Port()),
		ScrapperBaseURL: fmt.Sprintf("http://%s:%s", scrOrLocalhost(scrHost), scrPort.Port()),
		network:         network,
		bot:             bot,
		scrapper:        scr,
		http:            httpClient,
		projectRoot:     root,
	}
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
