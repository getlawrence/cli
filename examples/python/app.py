try:
    # Initialize OTEL if generated otel.py exists
    import otel  # noqa: F401
except Exception:
    pass
from flask import Flask

app = Flask(__name__)

@app.route('/')
def hello():
        return "Hello, from Python!"

@app.route('/health')
def health():
        return {"status": "healthy"}

if __name__ == '__main__':
    app.run(debug=True, port=5000)
