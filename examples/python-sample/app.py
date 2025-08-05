from flask import Flask
from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.exporter.console import ConsoleSpanExporter
from opentelemetry.sdk.trace.export import BatchSpanProcessor

# Initialize OpenTelemetry
trace.set_tracer_provider(TracerProvider())
tracer = trace.get_tracer(__name__)

# Configure console exporter
console_exporter = ConsoleSpanExporter()
span_processor = BatchSpanProcessor(console_exporter)
trace.get_tracer_provider().add_span_processor(span_processor)

app = Flask(__name__)

@app.route('/')
def hello():
    with tracer.start_as_current_span("hello-handler"):
        return "Hello, OpenTelemetry from Python!"

@app.route('/health')
def health():
    with tracer.start_as_current_span("health-check"):
        return {"status": "healthy"}

if __name__ == '__main__':
    app.run(debug=True, port=5000)
