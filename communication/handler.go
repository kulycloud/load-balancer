package communication

import (
	"context"
	commonCommunication "github.com/kulycloud/common/communication"
	"github.com/kulycloud/common/logging"
	protoCommon "github.com/kulycloud/protocol/common"
	protoLoadBalancer "github.com/kulycloud/protocol/load-balancer"
)

var _ protoLoadBalancer.LoadBalancerServer = &LoadBalancerHandler{}

var logger = logging.GetForComponent("handler")

type LoadBalancerHandler struct {
	protoLoadBalancer.UnimplementedLoadBalancerServer
	Storage *commonCommunication.StorageCommunicator
}

func NewLoadBalancerHandler() *LoadBalancerHandler {
	return &LoadBalancerHandler{
		Storage: commonCommunication.NewEmptyStorageCommunicator(),
	}
}

func (handler *LoadBalancerHandler) Register(listener *commonCommunication.Listener) {
	protoLoadBalancer.RegisterLoadBalancerServer(listener.Server, handler)
}

func (handler *LoadBalancerHandler) SetEndpoints(ctx context.Context, endpoints *protoCommon.EndpointList) (*protoCommon.Empty, error) {
	logger.Infow("Got endpoints", "endpoints", endpoints.Endpoints)

	err := globalConnectionCache.setEndpoints(ctx, endpoints.Endpoints)

	return &protoCommon.Empty{}, err
}

func (handler *LoadBalancerHandler) SetStorageEndpoints(ctx context.Context, endpoints *protoCommon.EndpointList) (*protoCommon.Empty, error) {
	logger.Infow("Got endpoints", "endpoints", endpoints.Endpoints)
	var comm *commonCommunication.ComponentCommunicator = nil
	var err error = nil

	if endpoints.Endpoints != nil && len(endpoints.Endpoints) > 0 {
		comm, err = commonCommunication.NewComponentCommunicatorFromEndpoint(endpoints.Endpoints[0])

		if err == nil {
			err = comm.Ping(ctx)
			if err != nil {
				// We cannot ping the new storage. We should explicitly set it to nil
				comm = nil
			}
		} else {
			comm = nil
		}
	}

	handler.Storage.UpdateComponentCommunicator(comm)
	handler.Storage.Endpoints = endpoints.Endpoints

	if err != nil {
		logger.Warnw("Error registering new storage endpoints", "endpoints", endpoints.Endpoints)
		return nil, err
	}
	logger.Infow("Registered new storage endpoints", "endpoints", endpoints.Endpoints)

	return &protoCommon.Empty{}, nil
}


