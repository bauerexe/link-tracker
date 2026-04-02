package bot

import "go.uber.org/fx"

var Module = fx.Options(
	ConfigModule,
	LoggerModule,
	GRPCModule,
	AppModule,
)
