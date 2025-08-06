from flask import Flask, jsonify
import requests
from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.exporter.jaeger.thrift import JaegerExporter
from opentelemetry.instrumentation.flask import FlaskInstrumentor
from opentelemetry.instrumentation.requests import RequestsInstrumentor

# Initialize OpenTelemetry
trace.set_tracer_provider(TracerProvider())

# Configure Jaeger exporter
jaeger_exporter = JaegerExporter(
    agent_host_name="localhost",
    agent_port=6831,
)

# Add span processor
span_processor = BatchSpanProcessor(jaeger_exporter)
trace.get_tracer_provider().add_span_processor(span_processor)

app = Flask(__name__)

# Instrument Flask and requests
FlaskInstrumentor().instrument_app(app)
RequestsInstrumentor().instrument()

@app.route('/')
def hello():
    tracer = trace.get_tracer(__name__)
    with tracer.start_as_current_span("hello-span"):
        return jsonify({"message": "Hello World!"})

@app.route('/api/data')
def get_data():
    tracer = trace.get_tracer(__name__)
    with tracer.start_as_current_span("get-data-span"):
        # Make an external request
        response = requests.get("https://jsonplaceholder.typicode.com/posts/1")
        return jsonify(response.json())

if __name__ == '__main__':
    app.run(debug=True, port=5000)
