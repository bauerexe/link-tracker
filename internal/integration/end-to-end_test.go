package integration

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	tcnetwork "github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestEndToEnd(t *testing.T) {
	testcontainers.SkipIfProviderIsNotHealthy(t)

	env := mustStartE2EEnv(t)
	defer env.Close(t)

	t.Run("Test1_Bot_CorrectUpdate_2000", func(t *testing.T) {
		testBotCorrectUpdate2000(t, env)
	})

	t.Run("Test2_Bot_IncorrectUpdate_Not200", func(t *testing.T) {
		testBotIncorrectUpdateNot200(t, env)
	})

	t.Run("Test3_1_Scrapper_AddAndGetLink", func(t *testing.T) {
		testScrapperAddAndGetLink(t, env)
	})

	t.Run("Test3_2_Scrapper_AddThenDeleteLink", func(t *testing.T) {
		testScrapperAddThenDeleteLink(t, env)
	})

	t.Run("Test3_3_Scrapper_DeleteFromNonexistentChat_Not200_AndLinkStillThere", func(t *testing.T) {
		testScrapperDeleteFromNonexistentChatNot200AndLinkStillThere(t, env)
	})

	t.Run("Test3_4_Scrapper_AddLinkToNonexistentChat_Not200", func(t *testing.T) {
		testScrapperAddLinkToNonexistentChatNot200(t, env)
	})

	t.Run("Test3_5_Scrapper_DeletedChat_CannotAddLink", func(t *testing.T) {
		testScrapperDeletedChatCannotAddLink(t, env)
	})

	t.Run("Test3_6_Scrapper_DeleteNonexistentChat_404", func(t *testing.T) {
		testScrapperDeleteNonexistentChat404(t, env)
	})
}

func testBotCorrectUpdate2000(t *testing.T, env *e2eEnv) {
	t.Helper()

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
	defer func(body io.ReadCloser) {
		_ = body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		env.DumpLogs(t)
		t.Fatalf("expected 200 OK, got %d, body=%s", resp.StatusCode, string(b))
	}
}

func testBotIncorrectUpdateNot200(t *testing.T, env *e2eEnv) {
	t.Helper()

	body := map[string]any{
		"id":        "oops",
		"url":       "not-a-uri",
		"tgChatIds": "also-wrong-type",
	}

	resp, err := env.doJSON("POST", env.BotBaseURL+"/updates", nil, body)
	mustNoErr(t, err)
	defer func(body io.ReadCloser) {
		_ = body.Close()
	}(resp.Body)

	if resp.StatusCode == http.StatusOK {
		t.Fatalf("expected non-200, got 200")
	}
}

func testScrapperAddAndGetLink(t *testing.T, env *e2eEnv) {
	t.Helper()

	chatID := int64(101)

	resp, err := env.doNoBody("POST", fmt.Sprintf("%s/tg-chat/%d", env.ScrapperBaseURL, chatID))
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
		"Tg-Chat-Id": strconv.FormatInt(chatID, 10),
	}, addReq)
	mustNoErr(t, err)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var list struct {
		Links []LinkItem `json:"links"`
		Size  int        `json:"size"`
	}
	resp, err = env.doJSON("GET", env.ScrapperBaseURL+"/links", map[string]string{
		"Tg-Chat-Id": strconv.FormatInt(chatID, 10),
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
}

func testScrapperAddThenDeleteLink(t *testing.T, env *e2eEnv) {
	t.Helper()

	chatID := int64(102)
	link := "https://github.com/golang/go"

	resp, err := env.doNoBody("POST", fmt.Sprintf("%s/tg-chat/%d", env.ScrapperBaseURL, chatID))
	mustNoErr(t, err)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp, err = env.doJSON("POST", env.ScrapperBaseURL+"/links", map[string]string{
		"Tg-Chat-Id": strconv.FormatInt(chatID, 10),
	}, map[string]any{
		"chatId": chatID,
		"link":   link,
	})
	mustNoErr(t, err)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp, err = env.doJSON("DELETE", env.ScrapperBaseURL+"/links", map[string]string{
		"Tg-Chat-Id": strconv.FormatInt(chatID, 10),
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
		"Tg-Chat-Id": strconv.FormatInt(chatID, 10),
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
}

func testScrapperDeleteFromNonexistentChatNot200AndLinkStillThere(t *testing.T, env *e2eEnv) {
	t.Helper()

	chatID := int64(103)
	link := "https://example.com"

	resp, err := env.doNoBody("POST", fmt.Sprintf("%s/tg-chat/%d", env.ScrapperBaseURL, chatID))
	mustNoErr(t, err)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp, err = env.doJSON("POST", env.ScrapperBaseURL+"/links", map[string]string{
		"Tg-Chat-Id": strconv.FormatInt(chatID, 10),
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
		"Tg-Chat-Id": strconv.FormatInt(chatID, 10),
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
}

func testScrapperAddLinkToNonexistentChatNot200(t *testing.T, env *e2eEnv) {
	t.Helper()

	chatID := int64(104)

	resp, err := env.doNoBody("POST", fmt.Sprintf("%s/tg-chat/%d", env.ScrapperBaseURL, chatID))
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
}

func testScrapperDeletedChatCannotAddLink(t *testing.T, env *e2eEnv) {
	t.Helper()

	chatID := int64(105)

	resp, err := env.doNoBody("POST", fmt.Sprintf("%s/tg-chat/%d", env.ScrapperBaseURL, chatID))
	mustNoErr(t, err)
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp, err = env.doNoBody("DELETE", fmt.Sprintf("%s/tg-chat/%d", env.ScrapperBaseURL, chatID))
	mustNoErr(t, err)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp, err = env.doJSON("POST", env.ScrapperBaseURL+"/links", map[string]string{
		"Tg-Chat-Id": strconv.FormatInt(chatID, 10),
	}, map[string]any{"link": "https://example.net"})
	mustNoErr(t, err)
	_ = resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("expected non-200, got 200")
	}
}

func testScrapperDeleteNonexistentChat404(t *testing.T, env *e2eEnv) {
	t.Helper()

	chatID := int64(999999)
	resp, err := env.doNoBody("DELETE", fmt.Sprintf("%s/tg-chat/%d", env.ScrapperBaseURL, chatID))
	mustNoErr(t, err)
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d", resp.StatusCode)
	}
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
	const mode = 0o644
	commonFiles := []testcontainers.ContainerFile{
		{
			HostFilePath:      envPath,
			ContainerFilePath: "/app/app.env",
			FileMode:          mode,
		},
	}
	pgReq := testcontainers.ContainerRequest{
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
			WithOccurrence(2).
			WithStartupTimeout(30 * time.Second),
	}

	pg, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: pgReq,
		Started:          true,
	})
	mustNoErr(t, err)

	flywayReq := testcontainers.ContainerRequest{
		Image:    "flyway/flyway:10",
		Networks: []string{networkName},
		NetworkAliases: map[string][]string{
			networkName: {"flyway"},
		},
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      filepath.Join(root, "migrations", "V1__init.sql"),
				ContainerFilePath: "/flyway/sql/V1__init.sql",
				FileMode:          mode,
			},
			{
				HostFilePath:      filepath.Join(root, "migrations", "V2__indexes.sql"),
				ContainerFilePath: "/flyway/sql/V2__indexes.sql",
				FileMode:          mode,
			},
			{
				HostFilePath:      filepath.Join(root, "migrations", "V3__github.sql"),
				ContainerFilePath: "/flyway/sql/V3__github.sql",
				FileMode:          mode,
			},
		},
		Env: map[string]string{
			"FLYWAY_URL":             "jdbc:postgresql://postgres:5432/link_tracker",
			"FLYWAY_USER":            "postgres",
			"FLYWAY_PASSWORD":        "postgres",
			"FLYWAY_CONNECT_RETRIES": "60",
		},
		Cmd:        []string{"-locations=filesystem:/flyway/sql", "migrate"},
		WaitingFor: wait.ForExit().WithExitTimeout(60 * time.Second),
	}

	flyway, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: flywayReq,
		Started:          true,
	})
	mustNoErr(t, err)
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
			"SECONDS_INTERVAL_CHECK": "30",
			"GITHUB_TOKEN":           "",
			"STACK_OVERFLOW_KEY":     "",
			"KAFKA_ENABLED":          "false",
		},
	}

	scr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: scrReq,
		Started:          false,
	})
	mustNoErr(t, err)
	if err = scr.Start(ctx); err != nil {
		dumpContainerLogs(ctx, t, pg, "postgres")
		dumpContainerLogs(ctx, t, flyway, "flyway")
		dumpContainerLogs(ctx, t, scr, "scrapper")
		_ = scr.Terminate(ctx)
		_ = network.Remove(ctx)
		t.Fatalf("start scrapper container: %v", err)
	}
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
		WaitingFor: wait.ForAll(
			wait.ForLog(`"msg":"starting bot"`),
		).WithDeadline(40 * time.Second), Env: map[string]string{
			"BOT_ADDR_HTTP":        "0.0.0.0:8082",
			"BOT_ADDR_GRPC":        "0.0.0.0:50052",
			"SCRAPPER_ADDR_GRPC":   "scrapper:50051",
			"APP_TELEGRAM_TOKEN":   "dummy",
			"BOT_DISABLE_TELEGRAM": "true",
			"KAFKA_ENABLED":        "false",
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
	time.Sleep(1 * time.Second)
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
