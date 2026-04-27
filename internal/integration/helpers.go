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

	"github.com/testcontainers/testcontainers-go"
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

func (e *e2eEnv) doNoBody(method, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(context.Background(), method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request without body: %w", err)
	}
	resp, err := e.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request without body: %w", err)
	}

	return resp, nil
}

func (e *e2eEnv) doJSON(method, url string, headers map[string]string, body any) (*http.Response, error) {
	var buf *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal json body: %w", err)
		}
		buf = bytes.NewReader(b)
	} else {
		buf = bytes.NewReader(nil)
	}

	req, err := http.NewRequestWithContext(context.Background(), method, url, buf)
	if err != nil {
		return nil, fmt.Errorf("create json request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := e.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do json request: %w", err)
	}

	return resp, nil
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
	defer func() {
		if closeErr := r.Close(); closeErr != nil {
			t.Logf("[%s] cannot close logs reader: %v", name, closeErr)
		}
	}()

	b, _ := io.ReadAll(r)
	t.Logf("=== %s logs ===\n%s\n=== end %s logs ===", name, string(b), name)
}

func startPostgres(ctx context.Context, networkName string) (testcontainers.Container, error) {
	const o = 2
	const i = 30
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		Started: true,
		ContainerRequest: testcontainers.ContainerRequest{
			Image:    "postgres:17",
			Networks: []string{networkName},
			NetworkAliases: map[string][]string{
				networkName: {"postgres"},
			},
			Env: map[string]string{
				"POSTGRES_DB":       "link_tracker",
				"POSTGRES_USER":     "postgres",
				"POSTGRES_PASSWORD": "postgres",
			},
			WaitingFor: wait.ForLog("database system is ready to accept connections").
				WithOccurrence(o).
				WithStartupTimeout(i * time.Second),
		},
	})
	if err != nil {
		return c, fmt.Errorf("start postgres: %w", err)
	}
	return c, nil
}

func startFlyway(ctx context.Context, root, networkName string) (testcontainers.Container, error) {
	const i = 60
	const mode = 0o644
	cont, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		Started: true,
		ContainerRequest: testcontainers.ContainerRequest{
			Image:          "flyway/flyway:10",
			Networks:       []string{networkName},
			NetworkAliases: map[string][]string{networkName: {"flyway"}},
			Files: []testcontainers.ContainerFile{
				{HostFilePath: filepath.Join(root, "migrations", "V1__init.sql"),
					ContainerFilePath: "/flyway/sql/V1__init.sql",
					FileMode:          mode,
				},
				{HostFilePath: filepath.Join(root, "migrations", "V2__indexes.sql"),
					ContainerFilePath: "/flyway/sql/V2__indexes.sql",
					FileMode:          mode},
				{HostFilePath: filepath.Join(root, "migrations", "V3__github.sql"),
					ContainerFilePath: "/flyway/sql/V3__github.sql",
					FileMode:          mode},
				{HostFilePath: filepath.Join(root, "migrations", "V4__outbox_messages.sql"),
					ContainerFilePath: "/flyway/sql/V4__outbox_messages.sql",
					FileMode:          mode}},
			Env: map[string]string{
				"FLYWAY_URL":             "jdbc:postgresql://postgres:5432/link_tracker",
				"FLYWAY_USER":            "postgres",
				"FLYWAY_PASSWORD":        "postgres",
				"FLYWAY_CONNECT_RETRIES": "60"},
			Cmd:        []string{"-locations=filesystem:/flyway/sql", "migrate"},
			WaitingFor: wait.ForExit().WithExitTimeout(i * time.Second)}})
	if err != nil {
		return nil, fmt.Errorf("start postgres: %w", err)
	}
	return cont, nil
}

func startKafka(ctx context.Context, networkName, id, alias string) (testcontainers.Container, error) {
	const i = 120
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		Started: true,
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "confluentinc/cp-kafka:7.3.2",
			ExposedPorts: []string{"19092/tcp"},
			Networks:     []string{networkName},
			NetworkAliases: map[string][]string{
				networkName: {alias},
			},
			Env: map[string]string{
				"KAFKA_BROKER_ID":                                id,
				"KAFKA_ZOOKEEPER_CONNECT":                        "zookeeper:2181",
				"KAFKA_LISTENERS":                                "INTERNAL://:19092",
				"KAFKA_ADVERTISED_LISTENERS":                     "INTERNAL://" + alias + ":19092",
				"KAFKA_LISTENER_SECURITY_PROTOCOL_MAP":           "INTERNAL:PLAINTEXT",
				"KAFKA_INTER_BROKER_LISTENER_NAME":               "INTERNAL",
				"KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR":         "1",
				"KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR": "1",
				"KAFKA_TRANSACTION_STATE_LOG_MIN_ISR":            "1",
				"KAFKA_DEFAULT_REPLICATION_FACTOR":               "1",
				"KAFKA_MIN_INSYNC_REPLICAS":                      "1",
			},
			WaitingFor: wait.ForLog("[KafkaServer id=1] started").
				WithStartupTimeout(i * time.Second),
		},
	})
	if err != nil {
		return c, fmt.Errorf("start kafka: %w", err)
	}
	return c, nil
}

func startZookeeper(ctx context.Context, networkName string) (testcontainers.Container, error) {
	const i = 60
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		Started: true,
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "confluentinc/cp-zookeeper:7.3.2",
			ExposedPorts: []string{"2181/tcp"},
			Networks:     []string{networkName},
			NetworkAliases: map[string][]string{
				networkName: {"zookeeper"},
			},
			Env: map[string]string{
				"ZOOKEEPER_CLIENT_PORT": "2181",
				"ZOOKEEPER_TICK_TIME":   "2000",
			},
			WaitingFor: wait.ForListeningPort("2181/tcp").
				WithStartupTimeout(i * time.Second),
		},
	})
	if err != nil {
		return c, fmt.Errorf("start zookeeper: %w", err)
	}
	return c, nil
}

func startInitKafka(ctx context.Context, networkName string) (testcontainers.Container, error) {
	const i = 60
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		Started: true,
		ContainerRequest: testcontainers.ContainerRequest{
			Image:    "confluentinc/cp-kafka:7.3.2",
			Networks: []string{networkName},
			Cmd: []string{"/bin/bash", "-c", `
set -e

for i in {1..30}; do
  kafka-topics --bootstrap-server kafka-1:19092 --list && break
  sleep 1
done

kafka-topics --bootstrap-server kafka-1:19092 --create --if-not-exists \
  --topic notifiers \
  --partitions 1 \
  --replication-factor 1

kafka-topics --bootstrap-server kafka-1:19092 --describe --topic notifiers
`},
			WaitingFor: wait.ForExit().WithExitTimeout(i * time.Second),
		},
	})
	if err != nil {
		return c, fmt.Errorf("start init kafka: %w", err)
	}
	return c, nil
}
