package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/trace"
)

func main() {
	// Initialize tracer
	tp, err := initTracer()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()

	r := gin.Default()

	// Add OpenTelemetry middleware
	r.Use(gin.WrapH(otelhttp.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), "gin-server")))

	r.GET("/", func(c *gin.Context) {
		tracer := otel.Tracer("example-tracer")
		_, span := tracer.Start(c.Request.Context(), "hello-span")
		defer span.End()

		c.JSON(200, gin.H{
			"message": "Hello World!",
		})
	})

	r.Run(":8080")
}

func initTracer() (*trace.TracerProvider, error) {
	exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint("http://localhost:14268/api/traces")))
	if err != nil {
		return nil, err
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
	)
	otel.SetTracerProvider(tp)
	return tp, nil
}
