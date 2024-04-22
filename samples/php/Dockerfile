FROM php:8 as base
WORKDIR /app
COPY . /app

####################################

FROM base AS dev

RUN apt-get update && apt-get install -y zip git libzip-dev && \
  pecl install xdebug && echo 'zend_extension="xdebug.so"' > /usr/local/etc/php/conf.d/xdebug.ini && \
  pecl install zip && echo 'extension=zip.so' > /usr/local/etc/php/conf.d/zip.ini && \
  php -r "copy('https://getcomposer.org/installer', 'composer-setup.php');" && \
  php composer-setup.php --install-dir /usr/local/bin && \
  ln -s /usr/local/bin/composer.phar /usr/local/bin/composer && \
  unlink composer-setup.php && \
  /usr/local/bin/composer global require laravel/installer

COPY xdebug.ini /usr/local/etc/php/conf.d/xdebug.ini
COPY php.ini-development /usr/local/etc/php/php.ini

ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/root/.composer/vendor/bin/:/okteto/vendor/bin/

####################################

FROM base AS prod

EXPOSE 8080
CMD [ "php", "-S", "0.0.0.0:8080" ]
