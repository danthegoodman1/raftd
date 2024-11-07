package env

import (
	"github.com/danthegoodman1/raftd/utils"
	"os"
)

var (
	Env                  = os.Getenv("ENV")
	TracingServiceName   = os.Getenv("TRACING_SERVICE_NAME")
	OLTPEndpoint         = os.Getenv("OLTP_ENDPOINT")
	HTTPListenAddr       = utils.GetEnvOrDefault("HTTP_LISTEN_ADDR", ":9090")
	RaftListenAddr       = utils.GetEnvOrDefault("RAFT_LISTEN_ADDR", "0.0.0.0:9091")
	MetricsAPIListenAddr = utils.GetEnvOrDefault("METRICS_LISTEN_ADDR", ":9092")

	RaftInitialMembers = os.Getenv("RAFT_INITIAL_MEMBERS") // csv of id=aadr pairs like 1=localhost:6000,2=localhost:6001,3=localhost:6002

	ApplicationURL       = utils.GetEnvOrDefault("APP_URL", "http://localhost:8080") // where the application can be reached, required
	ReplicaID            = uint64(utils.GetEnvOrDefaultInt("REPLICA_ID", 0))
	RaftSync             = utils.GetEnvOrDefaultInt("RAFT_SYNC", 0) == 1
	RaftStorageDirectory = utils.GetEnvOrDefault("RAFT_DIR", "_raft")
)
