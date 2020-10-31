package config

import (
	commonConfig "github.com/kulycloud/common/config"
)

type Config struct {
	Port uint32 `configName:"port"`
}

var GlobalConfig = &Config{}

func ParseConfig() error {
	parser := commonConfig.NewParser()
	parser.AddProvider(commonConfig.NewCliParamProvider())
	parser.AddProvider(commonConfig.NewEnvironmentVariableProvider())

	return parser.Populate(GlobalConfig)
}
