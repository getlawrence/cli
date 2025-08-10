FROM python:3.12-slim
WORKDIR /app
ENV PIP_NO_CACHE_DIR=1
COPY requirements.txt ./
RUN pip install --upgrade pip && pip install --no-cache-dir -r requirements.txt
COPY . .
ENV FLASK_APP=app.py
ENV OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318/v1/traces
EXPOSE 5000
CMD ["python", "app.py"]

