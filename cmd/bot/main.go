package main

import (
	"go.uber.org/fx"

	di "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/di/bot"
)

func main() {
	fx.New(
		di.Module,
	).Run()
}
