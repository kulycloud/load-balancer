package main

import (
	commonCommunication "github.com/kulycloud/common/communication"
	"github.com/kulycloud/common/logging"
	"github.com/kulycloud/load-balancer/communication"
	"github.com/kulycloud/load-balancer/config"
)

var logger = logging.GetForComponent("init")

func main() {
	defer logging.Sync()

	err := config.ParseConfig()
	if err != nil {
		logger.Fatalw("Error parsing config", "error", err)
	}
	logger.Infow("Finished parsing config", "config", config.GlobalConfig)


	logger.Info("Starting listener")
	listener := commonCommunication.NewListener(logging.GetForComponent("listener"))
	if err = listener.Setup(config.GlobalConfig.Port); err != nil {
		logger.Panicw("error initializing listener", "error", err)
	}

	handler := communication.NewLoadBalancerHandler()
	handler.Register(listener)

	if err = listener.Serve(); err != nil {
		logger.Panicw("error serving listener", "error", err)
	}

}
