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
		if err = <-listener.Serve(); err != nil {
			logger.Panicw("error serving listener", "error", err)
		}
	}()

	Storage = listener.Storage

	communication.InitConnectionCache(context.Background())

	srv, err := commonHttp.NewServer(config.GlobalConfig.HttpPort, handleFunc)
	if err != nil {
		logger.Panicw("error creating http server", "error", err)
	}
	err = srv.Serve()
	if err != nil {
		logger.Panicw("error serving http server", "error", err)
	}
}

func handleFunc(ctx context.Context, request *commonHttp.Request) *commonHttp.Response {
	var res *commonHttp.Response
	if Storage.Ready() {
		step, err := Storage.GetPopulatedRouteStepByUID(ctx, request.KulyData.RouteUid, request.KulyData.StepUid)
		if err != nil {
			logger.Warnw("could not get next step", "error", err)
			res = commonHttp.NewResponse()
			res.Status = 400
			return res
		}
		request.KulyData.Step = step
		res, err = communication.ProcessRequest(ctx, request)
		if err != nil {
			logger.Warnw("could not process request", "error", err)
			res = commonHttp.NewResponse()
			res.Status = 500
			return res
		}
		logger.Debugw("request processed", "request", request, "response", res)
		return res
	}
	logger.Warn("Storage not ready")
	res = commonHttp.NewResponse()
	res.Status = 500
	return res
}
