package main

import (
	"log"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
)

func main() {
	// Create a tracer
	tracer := otel.Tracer("example-service")

	// Create HTTP handler with OpenTelemetry instrumentation
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Start a span
		_, span := tracer.Start(ctx, "handle-request")
		defer span.End()

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, OpenTelemetry!"))
	})

	// Wrap handler with OpenTelemetry middleware
	wrappedHandler := otelhttp.NewHandler(handler, "hello-handler")

	http.Handle("/", wrappedHandler)

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
