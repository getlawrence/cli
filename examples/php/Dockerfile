FROM php:8.2-cli

WORKDIR /app

COPY . /app

# Install system deps for composer (git/unzip), then composer, then install deps if composer.json exists
RUN apt-get update \
 && apt-get install -y --no-install-recommends git unzip ca-certificates \
 && rm -rf /var/lib/apt/lists/* \
 && php -r "copy('https://getcomposer.org/installer', 'composer-setup.php');" \
 && php composer-setup.php --install-dir=/usr/local/bin --filename=composer \
 && rm composer-setup.php \
 && if [ -f composer.json ]; then composer install --no-interaction --prefer-dist --no-dev --no-progress; fi

# Expose service port
EXPOSE 8000

# Run PHP built-in server to serve index.php
CMD ["php", "-S", "0.0.0.0:8000", "-t", "/app"]


