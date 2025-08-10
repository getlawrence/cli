FROM ruby:3.3

WORKDIR /app

COPY Gemfile /app/Gemfile
RUN bundle install

COPY . /app

EXPOSE 4567

ENV OTEL_EXPORTER_OTLP_ENDPOINT=
ENV OTEL_SERVICE_NAME=examples-ruby

CMD ["ruby", "app.rb", "-o", "0.0.0.0", "-p", "4567"]


