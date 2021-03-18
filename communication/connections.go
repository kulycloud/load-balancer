package communication

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/kulycloud/load-balancer/config"

	commonHttp "github.com/kulycloud/common/http"
	protoCommon "github.com/kulycloud/protocol/common"
)

var ErrNoValidCommunicator = errors.New("no valid communicator")

var globalConnectionCache *connectionCache

func InitConnectionCache(ctx context.Context) {
	globalConnectionCache = &connectionCache{}
	go func() {
	cacheUpdateLoop:
		for {
			select {
			case <-ctx.Done():
				break cacheUpdateLoop
			default:
				globalConnectionCache.update(ctx)
				time.Sleep(time.Duration(config.GlobalConfig.UpdateTimeout) * time.Second)
			}
		}
	}()
}

func ProcessRequest(ctx context.Context, request *commonHttp.Request) (*commonHttp.Response, error) {
	// if route revisions are added then this will select the proper cache
	return globalConnectionCache.processRequest(ctx, request)
}

// if route revisions are added this can be used
// in a map of revisions
// connections contains entries for valid connections
// sorted by certain criteria (e.g. response time)
// connections[0] -> the best
// invalid endpoints contains endpoint for which
// no valid connection could be created
type connectionCache struct {
	mutex            sync.RWMutex
	connections      []*connection
	invalidEndpoints []*protoCommon.Endpoint
}

// stores the last recorded response time to a ping
// along with the communicator
// could be extended in the future if more criteria
// are needed
type connection struct {
	endpoint     *protoCommon.Endpoint
	communicator *commonHttp.Communicator
}

// create valid connection if possible
func newConnection(ctx context.Context, endpoint *protoCommon.Endpoint) (*connection, error) {
	com, err := commonHttp.NewCommunicatorFromEndpoint(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	connection := &connection{
		endpoint:     endpoint,
		communicator: com,
	}
	return connection, nil
}

// update evaluation criteria values (e.g. response time)
func (con *connection) update(ctx context.Context) error {
	return con.communicator.Ping(ctx)
}

// update the connection metrics by which the connections will be sorted
// loop over valid connections and try to process request
// first connection is the best based on criteria
// if it returns an error try the next connection
func (cc *connectionCache) processRequest(ctx context.Context, request *commonHttp.Request) (*commonHttp.Response, error) {
	cc.mutex.RLock()
	defer cc.mutex.RUnlock()

	for _, connection := range cc.connections {
		res, err := connection.communicator.ProcessRequest(ctx, request)
		if err == nil {
			return res, nil
		}
	}
	return nil, ErrNoValidCommunicator
}

// create connections for new set of endpoints
func (cc *connectionCache) setEndpoints(ctx context.Context, endpoints []*protoCommon.Endpoint) error {
	connections, invalidEndpoints := createConnections(ctx, endpoints)
	if len(connections) < 1 {
		logger.Errorw("load balancer will not function properly", "error", ErrNoValidCommunicator)
	}

	cc.mutex.Lock()
	defer cc.mutex.Unlock()

	cc.connections = connections
	cc.invalidEndpoints = invalidEndpoints
	sort.Sort(cc)
	return nil
}

// update cache
func (cc *connectionCache) update(ctx context.Context) {
	newConnections, oldInvalidEndpoints := createConnections(ctx, cc.invalidEndpoints)
	oldConnections, newInvalidEndpoints := validateConnections(ctx, cc.connections)

	connections := append(oldConnections, newConnections...)
	invalidEndpoints := append(oldInvalidEndpoints, newInvalidEndpoints...)

	if len(cc.connections) < 1 {
		logger.Errorw("load balancer will not function properly", "error", ErrNoValidCommunicator)
	}

	cc.mutex.Lock()
	defer cc.mutex.Unlock()

	cc.connections = connections
	cc.invalidEndpoints = invalidEndpoints
	sort.Sort(cc)
}

// creates connections for every possible endpoint
// invalid endpoints are returned separatly
func createConnections(ctx context.Context, endpoints []*protoCommon.Endpoint) ([]*connection, []*protoCommon.Endpoint) {
	connections := make([]*connection, 0, len(endpoints))
	invalidEndpoints := make([]*protoCommon.Endpoint, 0)

	for _, endpoint := range endpoints {
		connection, err := newConnection(ctx, endpoint)
		if err != nil {
			invalidEndpoints = append(invalidEndpoints, endpoint)
		} else {
			connections = append(connections, connection)
		}
	}

	return connections, invalidEndpoints
}

// checks if provided connections are valid
// return slice of valid connection and slice of invalid endpoints
func validateConnections(ctx context.Context, connections []*connection) ([]*connection, []*protoCommon.Endpoint) {
	validConnections := make([]*connection, 0, len(connections))
	invalidEndpoints := make([]*protoCommon.Endpoint, 0)

	for _, connection := range connections {
		err := connection.update(ctx)
		if err != nil {
			invalidEndpoints = append(invalidEndpoints, connection.endpoint)
		} else {
			validConnections = append(validConnections, connection)
		}
	}

	return validConnections, invalidEndpoints
}

// sorting
// the connections will be sorted based on certain
// criteria. for that connectionCache implements the
// Sortable interface
func (cc *connectionCache) Len() int {
	return len(cc.connections)
}
func (cc *connectionCache) Swap(i, j int) {
	cc.connections[i], cc.connections[j] = cc.connections[j], cc.connections[i]
}
func (cc *connectionCache) Less(i, j int) bool {
	// could optionally also consider other metrics but lru should suffice
	return cc.connections[i].communicator.GetMetrics().LastUseTS < cc.connections[j].communicator.GetMetrics().LastUseTS
}
