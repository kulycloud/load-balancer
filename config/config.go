package config

import (
	commonConfig "github.com/kulycloud/common/config"
)

type Config struct {
	Port          uint32 `configName:"port"`
	HttpPort      uint32 `configName:"httpPort"`
	UpdateTimeout uint32 `configName:"updateTimeout" defaultValue:"2"`
}

var GlobalConfig = &Config{}

func ParseConfig() error {
	parser := commonConfig.NewParser()
	parser.AddProvider(commonConfig.NewCliParamProvider())
	parser.AddProvider(commonConfig.NewEnvironmentVariableProvider())

	return parser.Populate(GlobalConfig)
}
