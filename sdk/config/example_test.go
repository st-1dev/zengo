package config_test

import (
	"zengo/platform/sdk/config"
)

func ExampleNewLoader() {
	loader := config.NewLoader("configs")
	_ = loader
}
