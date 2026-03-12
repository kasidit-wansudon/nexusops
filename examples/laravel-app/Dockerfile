# ---- Composer Dependencies ----
FROM composer:2.7 AS vendor

WORKDIR /app

COPY composer.json composer.lock* ./
RUN composer install \
    --no-dev \
    --no-interaction \
    --no-scripts \
    --prefer-dist \
    --optimize-autoloader

# ---- Application Image ----
FROM php:8.3-fpm-alpine

# Install system dependencies and PHP extensions required by Laravel.
RUN apk add --no-cache \
        nginx \
        supervisor \
        curl \
        libpq-dev \
        libzip-dev \
        icu-dev \
        oniguruma-dev \
    && docker-php-ext-install \
        pdo_pgsql \
        pgsql \
        zip \
        intl \
        mbstring \
        opcache \
        pcntl \
    && docker-php-ext-enable opcache

# PHP production settings.
RUN mv "$PHP_INI_DIR/php.ini-production" "$PHP_INI_DIR/php.ini"
COPY docker/php/opcache.ini /usr/local/etc/php/conf.d/opcache.ini 2>/dev/null || true

# Nginx configuration.
COPY docker/nginx/default.conf /etc/nginx/http.d/default.conf 2>/dev/null || true

# Supervisor to run both nginx and php-fpm.
COPY docker/supervisor/supervisord.conf /etc/supervisor/conf.d/supervisord.conf 2>/dev/null || true

WORKDIR /var/www/html

# Copy vendor dependencies from the composer stage.
COPY --from=vendor /app/vendor ./vendor

# Copy the application source.
COPY . .

# Ensure storage and cache directories are writable.
RUN mkdir -p storage/framework/{cache,sessions,views} \
        storage/logs \
        bootstrap/cache \
    && chown -R www-data:www-data storage bootstrap/cache \
    && chmod -R 775 storage bootstrap/cache

# Cache config and routes for production (will succeed if artisan exists).
RUN php artisan config:cache 2>/dev/null || true \
    && php artisan route:cache 2>/dev/null || true \
    && php artisan view:cache 2>/dev/null || true

EXPOSE 8000

# Default command: run PHP's built-in server for simplicity in this example.
# In production, supervisor would start both nginx and php-fpm.
CMD ["php", "artisan", "serve", "--host=0.0.0.0", "--port=8000"]

HEALTHCHECK --interval=15s --timeout=5s --start-period=30s --retries=3 \
    CMD ["curl", "-f", "http://localhost:8000/api/health"]
