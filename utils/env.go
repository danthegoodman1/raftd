package utils

import "os"

var (
	Env                  = os.Getenv("ENV")
	TracingServiceName   = os.Getenv("TRACING_SERVICE_NAME")
	OLTPEndpoint         = os.Getenv("OLTP_ENDPOINT")
	HTTPListenAddr       = GetEnvOrDefault("HTTP_LISTEN_ADDR", ":9090")
	RaftListenAddr       = GetEnvOrDefault("RAFT_LISTEN_ADDR", "0.0.0.0:9091")
	MetricsAPIListenAddr = GetEnvOrDefault("METRICS_LISTEN_ADDR", ":9092")

	RaftPeers = os.Getenv("RAFT_PEERS") // csv of id=aadr pairs like 1=localhost:6000,2=localhost:6001,3=localhost:6002

	ApplicationURL       = GetEnvOrDefault("APP_URL", "http://localhost:8080") // where the application can be reached, required
	NodeID               = uint64(GetEnvOrDefaultInt("NODE_ID", 0))
	RaftSync             = GetEnvOrDefaultInt("RAFT_SYNC", 0) == 1
	RaftStorageDirectory = GetEnvOrDefault("RAFT_DIR", "_raft")
)
