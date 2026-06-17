package scrapper

import "go.uber.org/fx"

var Module = fx.Options(
	ConfigModule,
	LoggerModule,
	DBModule,
	CacheModule,
	MetricsModule,
	HTTPModule,
	KafkaModule,
	GRPCModule,
	RepositoryModule,
	AppModule,
	RunModule,
)
