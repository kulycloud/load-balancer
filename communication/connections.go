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

var ErrNoValidCommunicator = errors.New("no valid communicator found")

var globalConnectionCache *connectionCache

func InitConnectionCache() {
	globalConnectionCache = &connectionCache{}
	go func() {
		for {
			globalConnectionCache.update()
			time.Sleep(time.Duration(config.GlobalConfig.UpdateTimeout) * time.Second)
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
	mutex            sync.Mutex
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
	responseTime int64
}

// create valid connection if possible
func newConnection(endpoint *protoCommon.Endpoint) (*connection, error) {
	com, err := commonHttp.NewCommunicatorFromEndpoint(endpoint)
	if err != nil {
		return nil, err
	}
	connection := &connection{
		endpoint:     endpoint,
		communicator: com,
	}
	err = connection.update()
	return connection, err
}

// update evaluation criteria values (e.g. response time)
func (con *connection) update() error {
	start := time.Now()
	err := con.communicator.Ping(context.Background())
	duration := (int64)(time.Since(start))
	con.responseTime = duration
	return err
}

// loop over valid connections and try to process request
// first connection is the best based on criteria
// if it returns an error try the next connection
func (cc *connectionCache) processRequest(ctx context.Context, request *commonHttp.Request) (*commonHttp.Response, error) {
	cc.mutex.Lock()
	defer cc.mutex.Unlock()

	for _, connection := range cc.connections {
		res, err := connection.communicator.ProcessRequest(ctx, request)
		if err == nil {
			return res, nil
		}
	}
	return nil, ErrNoValidCommunicator
}

// create connections for new set of endpoints
// if at least 1 valid connection can be created
// then flush cache and add new connections
// and new set of invalidEndpoints
// otherwise keep old connections and return error
func (cc *connectionCache) setEndpoints(endpoints []*protoCommon.Endpoint) error {
	connections, invalidEndpoints := createConnections(endpoints)
	if len(connections) < 1 {
		return ErrNoValidCommunicator
	}

	cc.mutex.Lock()
	defer cc.mutex.Unlock()

	cc.connections = connections
	cc.invalidEndpoints = invalidEndpoints
	sort.Sort(cc)
	return nil
}

// update cache
func (cc *connectionCache) update() {
	cc.mutex.Lock()
	defer cc.mutex.Unlock()

	cc.updateInvalidEndpoints()
	cc.updateConnections()
	sort.Sort(cc)
}

// tries to create connections for previously
// invalid endpoints
// if successful, adds connection to connections
// and removes endpoint from invalidEndpoints
func (cc *connectionCache) updateInvalidEndpoints() {
	newInvalidEndpoints := make([]*protoCommon.Endpoint, 0, len(cc.invalidEndpoints))
	for _, endpoint := range cc.invalidEndpoints {
		connection, err := newConnection(endpoint)
		if err == nil {
			cc.connections = append(cc.connections, connection)
		} else {
			newInvalidEndpoints = append(newInvalidEndpoints, endpoint)
		}
	}
	cc.invalidEndpoints = newInvalidEndpoints
}

// updates previously valid connections
// if update not successful then remove from
// connections and add endpoint to invalidEndpoints
func (cc *connectionCache) updateConnections() {
	newConnections := make([]*connection, 0, len(cc.connections))
	for _, connection := range cc.connections {
		err := connection.update()
		if err == nil {
			newConnections = append(newConnections, connection)
		} else {
			cc.invalidEndpoints = append(cc.invalidEndpoints, connection.endpoint)
		}
	}
	cc.connections = newConnections
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
	return cc.connections[i].responseTime < cc.connections[j].responseTime
}

// creates connections for every possible endpoint
// invalid endpoints are returned separatly
func createConnections(endpoints []*protoCommon.Endpoint) ([]*connection, []*protoCommon.Endpoint) {
	connections := make([]*connection, 0, len(endpoints))
	invalidEndpoints := make([]*protoCommon.Endpoint, 0)

	for _, endpoint := range endpoints {
		connection, err := newConnection(endpoint)
		if err != nil {
			invalidEndpoints = append(invalidEndpoints, endpoint)
		} else {
			connections = append(connections, connection)
		}
	}

	return connections, invalidEndpoints
}
