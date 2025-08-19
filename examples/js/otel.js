// OpenTelemetry bootstrap for js (JavaScript/Node.js, CommonJS)

// Install packages:
// npm install @opentelemetry/api @opentelemetry/sdk-node @opentelemetry/exporter-trace-otlp-http

// Create file: otel.js and require it at process start

const { NodeSDK } = require("@opentelemetry/sdk-node");
const { OTLPTraceExporter } = require("@opentelemetry/exporter-trace-otlp-http");

// Resolve OTLP HTTP endpoint from environment if not explicitly set
let otlpUrl = undefined;
if (process.env.OTEL_EXPORTER_OTLP_TRACES_ENDPOINT) {
  otlpUrl = process.env.OTEL_EXPORTER_OTLP_TRACES_ENDPOINT;
} else if (process.env.OTEL_EXPORTER_OTLP_ENDPOINT) {
  const base = process.env.OTEL_EXPORTER_OTLP_ENDPOINT.replace(/\/$/, "");
  otlpUrl = `${base}/v1/traces`;
}

const sdk = new NodeSDK({
  traceExporter: new OTLPTraceExporter({...(otlpUrl ? { url: otlpUrl } : {}),
  }),
});

sdk.start();

// Emit a bootstrap span using API to ensure at least one span is produced
const api = require("@opentelemetry/api");
const tracer = api.trace.getTracer("bootstrap");
tracer.startActiveSpan("startup", (span) => { span.end(); });

process.on("SIGTERM", async () => { await sdk.shutdown(); });
