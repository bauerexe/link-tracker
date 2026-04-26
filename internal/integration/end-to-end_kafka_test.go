package integration

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	tcnetwork "github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestEndToEndKafka_ScrapperNotifiesBot(t *testing.T) {
	testcontainers.SkipIfProviderIsNotHealthy(t)

	env := mustStartE2EEnvKafka(t)
	defer env.Close(t)

	chatID := int64(777)
	link := "https://stackoverflow.com/questions/11227809/why-is-processing-a-sorted-array-faster-than-processing-an-unsorted-array"

	resp, err := env.doNoBody("POST", fmt.Sprintf("%s/tg-chat/%d", env.ScrapperBaseURL, chatID))
	mustNoErr(t, err)
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create chat: expected 200, got %d", resp.StatusCode)
	}

	resp, err = env.doJSON("POST", env.ScrapperBaseURL+"/links", map[string]string{
		"Tg-Chat-Id": strconv.FormatInt(chatID, 10),
	}, map[string]any{
		"link": link,
	})
	mustNoErr(t, err)
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("track link: expected 200, got %d", resp.StatusCode)
	}

	execPostgres(t, env.postgres, `
UPDATE links
SET last_updated_at = NOW() - INTERVAL '20 years',
    last_checked_at = NOW() - INTERVAL '20 years'
WHERE url = 'https://stackoverflow.com/questions/11227809/why-is-processing-a-sorted-array-faster-than-processing-an-unsorted-array';
`)

	deadline := time.Now().Add(10 * time.Second)

	for time.Now().Before(deadline) {
		logs := containerLogs(t, env.bot, "bot")

		if strings.Contains(logs, "dummy telegram message sent") {
			return
		}

	}

	env.DumpLogs(t)
	t.Fatalf("expected bot to receive notification from scrapper through kafka")
}

type e2eKafkaEnv struct {
	*e2eEnv

	postgres  testcontainers.Container
	flyway    testcontainers.Container
	zookeeper testcontainers.Container
	kafka1    testcontainers.Container
	initKafka testcontainers.Container
}

func (e *e2eKafkaEnv) Close(t *testing.T) {
	t.Helper()

	ctx := context.Background()

	if e.bot != nil {
		_ = e.bot.Terminate(ctx)
	}
	if e.scrapper != nil {
		_ = e.scrapper.Terminate(ctx)
	}
	if e.initKafka != nil {
		_ = e.initKafka.Terminate(ctx)
	}
	if e.kafka1 != nil {
		_ = e.kafka1.Terminate(ctx)
	}
	if e.zookeeper != nil {
		_ = e.zookeeper.Terminate(ctx)
	}
	if e.flyway != nil {
		_ = e.flyway.Terminate(ctx)
	}
	if e.postgres != nil {
		_ = e.postgres.Terminate(ctx)
	}
	if e.network != nil {
		_ = e.network.Remove(ctx)
	}
}

const (
	kafkaAlias = "kafka-1"
	kafkaAddr  = "kafka-1:19092"
	topicName  = "notifiers"
)

func mustStartE2EEnvKafka(t *testing.T) *e2eKafkaEnv {
	t.Helper()

	root := mustFindProjectRoot(t)
	ctx := context.Background()
	httpClient := &http.Client{Timeout: 10 * time.Second}

	network, err := tcnetwork.New(ctx)
	mustNoErr(t, err)
	networkName := network.Name

	envPath := filepath.Join(root, "app.env")
	commonFiles := []testcontainers.ContainerFile{{
		HostFilePath:      envPath,
		ContainerFilePath: "/app/app.env",
		FileMode:          0o644,
	}}

	var (
		pg, flyway testcontainers.Container
		zk, kafka1 testcontainers.Container
		initKafka  testcontainers.Container
	)

	errCh := make(chan error, 2)

	go func() {
		pg, err = startPostgres(ctx, networkName)
		if err != nil {
			errCh <- fmt.Errorf("start postgres: %w", err)
			return
		}

		flyway, err = startFlyway(ctx, root, networkName)
		if err != nil {
			errCh <- fmt.Errorf("start flyway: %w", err)
			return
		}

		errCh <- nil
	}()

	go func() {
		zk, err = startZookeeper(ctx, networkName)
		if err != nil {
			errCh <- fmt.Errorf("start zookeeper: %w", err)
			return
		}

		kafka1, err = startKafka(ctx, networkName, "1", kafkaAlias)
		if err != nil {
			errCh <- fmt.Errorf("start kafka: %w", err)
			return
		}

		initKafka, err = startInitKafka(ctx, networkName)
		if err != nil {
			errCh <- fmt.Errorf("init kafka: %w", err)
			return
		}

		errCh <- nil
	}()

	for range 2 {
		if err = <-errCh; err != nil {
			t.Fatal(err)
		}
	}

	scr := startScrapper(ctx, t, root, networkName, commonFiles, pg, flyway, zk, kafka1, initKafka)
	bot := startBot(ctx, t, root, networkName, commonFiles, scr)

	scrHost, err := scr.Host(ctx)
	mustNoErr(t, err)
	scrPort, err := scr.MappedPort(ctx, "8080/tcp")
	mustNoErr(t, err)

	botHost, err := bot.Host(ctx)
	mustNoErr(t, err)
	botPort, err := bot.MappedPort(ctx, "8082/tcp")
	mustNoErr(t, err)

	return &e2eKafkaEnv{
		e2eEnv: &e2eEnv{
			BotBaseURL:      fmt.Sprintf("http://%s:%s", scrOrLocalhost(botHost), botPort.Port()),
			ScrapperBaseURL: fmt.Sprintf("http://%s:%s", scrOrLocalhost(scrHost), scrPort.Port()),
			network:         network,
			bot:             bot,
			scrapper:        scr,
			http:            httpClient,
			projectRoot:     root,
		},
		postgres:  pg,
		flyway:    flyway,
		zookeeper: zk,
		kafka1:    kafka1,
		initKafka: initKafka,
	}
}

func containerLogs(t *testing.T, c testcontainers.Container, name string) string {
	t.Helper()

	ctx := context.Background()

	rc, err := c.Logs(ctx)
	if err != nil {
		t.Fatalf("read %s logs: %v", name, err)
	}
	defer rc.Close()

	b, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read %s logs body: %v", name, err)
	}

	return string(b)
}
func execPostgres(t *testing.T, pg testcontainers.Container, query string) {
	t.Helper()

	ctx := context.Background()

	code, out, err := pg.Exec(ctx, []string{
		"psql",
		"-U", "postgres",
		"-d", "link_tracker",
		"-c", query,
	})

	if err != nil {
		t.Fatalf("exec postgres: %v", err)
	}

	if code != 0 {
		t.Fatalf("exec postgres failed with code %d: %s", code, out)
	}
}

func startScrapper(
	ctx context.Context,
	t *testing.T,
	root, networkName string,
	commonFiles []testcontainers.ContainerFile,
	pg, flyway, zk, kafka1, initKafka testcontainers.Container,
) testcontainers.Container {
	t.Helper()

	req := testcontainers.ContainerRequest{
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
		WaitingFor: wait.ForAll(
			wait.ForLog("gateway listening at port"),
			wait.ForListeningPort("8080/tcp"),
			wait.ForListeningPort("50051/tcp"),
		).WithDeadline(60 * time.Second),
		Env: map[string]string{
			"SCRAPPER_ADDR_HTTP":     "0.0.0.0:8080",
			"SCRAPPER_ADDR_GRPC":     "0.0.0.0:50051",
			"BOT_ADDR_GRPC":          "bot:50052",
			"SECONDS_INTERVAL_CHECK": "4",
			"GITHUB_TOKEN":           "",
			"STACK_OVERFLOW_KEY":     "",

			"KAFKA_ENABLED": "true",
			"KAFKA_BROKERS": kafkaAddr,
			"KAFKA_TOPIC":   topicName,
		},
	}

	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          false,
	})
	mustNoErr(t, err)

	if err = c.Start(ctx); err != nil {
		dumpContainerLogs(ctx, t, pg, "postgres")
		dumpContainerLogs(ctx, t, flyway, "flyway")
		dumpContainerLogs(ctx, t, zk, "zookeeper")
		dumpContainerLogs(ctx, t, kafka1, "kafka-1")
		dumpContainerLogs(ctx, t, initKafka, "init-kafka")
		dumpContainerLogs(ctx, t, c, "scrapper")
		t.Fatalf("start scrapper container: %v", err)
	}

	return c
}

func startBot(
	ctx context.Context,
	t *testing.T,
	root, networkName string,
	commonFiles []testcontainers.ContainerFile,
	scr testcontainers.Container,
) testcontainers.Container {
	t.Helper()

	req := testcontainers.ContainerRequest{
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
		WaitingFor: wait.ForAll(
			wait.ForLog("starting kafka consumer"),
			wait.ForLog("bot run"),
		).WithDeadline(60 * time.Second),
		Env: map[string]string{
			"BOT_ADDR_HTTP":        "0.0.0.0:8082",
			"BOT_ADDR_GRPC":        "0.0.0.0:50052",
			"SCRAPPER_ADDR_GRPC":   "scrapper:50051",
			"APP_TELEGRAM_TOKEN":   "dummy",
			"BOT_DISABLE_TELEGRAM": "true",

			"KAFKA_ENABLED":        "true",
			"KAFKA_BROKERS":        kafkaAddr,
			"KAFKA_TOPIC":          topicName,
			"KAFKA_CONSUMER_GROUP": "notifiers-consumer",
		},
	}

	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          false,
	})
	mustNoErr(t, err)

	if err = c.Start(ctx); err != nil {
		dumpContainerLogs(ctx, t, scr, "scrapper")
		dumpContainerLogs(ctx, t, c, "bot")
		t.Fatalf("start bot container: %v", err)
	}

	return c
}
