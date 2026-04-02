package scrapper

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/services/github"
	repository "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/repository/services/postgres"
	"go.uber.org/fx"

	scrapperapp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
	ormrepo "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/repository/scrapper/postgres"
	rawrepo "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/repository/scrapper/rawpostgres"
)

var RepositoryModule = fx.Options(
	fx.Provide(
		newChatRepo,
		newLinkRepo,
		newGithubRepository,
	),
)

func newChatRepo(
	cfg *config.ScrapperConfig,
	pool *pgxpool.Pool,
) (scrapperapp.ChatRepository, error) {
	switch cfg.DBAccessType {
	case "sql":
		return rawrepo.NewChatRepository(pool), nil
	case "orm":
		return ormrepo.NewChatRepository(pool), nil
	default:
		return nil, config.ErrParseFile
	}
}

func newLinkRepo(
	cfg *config.ScrapperConfig,
	pool *pgxpool.Pool,
) (scrapperapp.LinkRepository, error) {
	switch cfg.DBAccessType {
	case "sql":
		return rawrepo.NewLinkRepository(pool), nil
	case "orm":
		return ormrepo.NewLinkRepository(pool), nil
	default:
		return nil, config.ErrParseFile
	}
}

func newGithubRepository(pool *pgxpool.Pool) github.Repository {
	return repository.NewGithubRepository(pool)
}
