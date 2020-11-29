package main

import (
	"context"

	commonCommunication "github.com/kulycloud/common/communication"
	commonHttp "github.com/kulycloud/common/http"
	"github.com/kulycloud/common/logging"
	"github.com/kulycloud/load-balancer/communication"
	"github.com/kulycloud/load-balancer/config"
)

var logger = logging.GetForComponent("init")

var Storage *commonCommunication.StorageCommunicator

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

	go func() {
		if err = listener.Serve(); err != nil {
			logger.Panicw("error serving listener", "error", err)
		}
	}()

	Storage = listener.Storage

	srv := commonHttp.NewServer(config.GlobalConfig.HttpPort, handleFunc)
	err = srv.Serve()
	if err != nil {
		logger.Panicw("error serving http server", "error", err)
	}
}

func handleFunc(request *commonHttp.Request) *commonHttp.Response {
	res := commonHttp.NewResponse()
	// could be replaced with a more sensible context later on
	ctx := context.Background()
	if Storage.Ready() {
		step, err := Storage.GetRouteStepByUID(ctx, request.KulyData.RouteUid, request.KulyData.StepUid)
		if err != nil {
			logger.Errorw("could not get next step", "error", err)
			res.Status = 400
			return res
		}
		request.KulyData.Step = step
		res, err = communication.ProcessRequest(ctx, request)
		if err != nil {
			logger.Errorw("could not process request", "error", err)
			res.Status = 500
			return res
		}
	}
	res.Status = 500
	return res
}
