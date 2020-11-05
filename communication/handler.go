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
}

func NewLoadBalancerHandler() *LoadBalancerHandler {
	return &LoadBalancerHandler{}
}

func (handler *LoadBalancerHandler) Register(listener *commonCommunication.Listener) {
	protoLoadBalancer.RegisterLoadBalancerServer(listener.Server, handler)
}

func (handler *LoadBalancerHandler) SetEndpoints(ctx context.Context, endpoints *protoCommon.EndpointList) (*protoCommon.Empty, error) {
	logger.Infow("Got endpoints", "endpoints", endpoints.Endpoints)
	return &protoCommon.Empty{}, nil
}
