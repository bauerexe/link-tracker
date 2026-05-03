package scrapper

import "go.uber.org/fx"

var Module = fx.Options(
	ConfigModule,
	LoggerModule,
	DBModule,
	CacheModule,
	HTTPModule,
	KafkaModule,
	GRPCModule,
	RepositoryModule,
	AppModule,
	RunModule,
)
