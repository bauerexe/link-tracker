package scrapper

import "go.uber.org/fx"

var Module = fx.Options(
	ConfigModule,
	LoggerModule,
	DBModule,
	HTTPModule,
	GRPCModule,
	RepositoryModule,
	AppModule,
	RunModule,
)
