FROM php:8.2-cli

WORKDIR /app

COPY . /app

# Expose service port
EXPOSE 8000

# Run PHP built-in server to serve index.php
CMD ["php", "-S", "0.0.0.0:8000", "-t", "/app"]


