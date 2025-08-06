module github.com/example/go-otel-app

go 1.19

require (
	go.opentelemetry.io/otel v1.15.0
	go.opentelemetry.io/otel/sdk v1.15.0
	go.opentelemetry.io/otel/exporters/jaeger v1.15.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.40.0
	github.com/gin-gonic/gin v1.9.0
	github.com/lib/pq v1.10.0
)
