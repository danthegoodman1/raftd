package utils

import "os"

var (
	Env                  = os.Getenv("ENV")
	TracingServiceName   = os.Getenv("TRACING_SERVICE_NAME")
	OLTPEndpoint         = os.Getenv("OLTP_ENDPOINT")
	MetricsAPIListenAddr = GetEnvOrDefault("METRICS_API_ADDR", ":8042")
	HTTPPort             = GetEnvOrDefault("HTTP_PORT", "8080")
	ApplicationURL       = os.Getenv("APP_URL") // where the application can be reached, required
)
