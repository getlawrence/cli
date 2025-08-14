FROM maven:3.9.6-eclipse-temurin-17 AS build
WORKDIR /workspace

# Copy pom.xml first to leverage Docker layer caching for dependencies
COPY pom.xml .
# Download dependencies (this layer will be cached if pom.xml doesn't change)
RUN mvn dependency:go-offline -B

# Copy source code
COPY src ./src
# Build the application
RUN mvn -q -e -DskipTests package

FROM eclipse-temurin:17
WORKDIR /app
COPY --from=build /workspace/target/java-sample-1.0-SNAPSHOT.jar /app/app.jar

EXPOSE 8082

ENV OTEL_EXPORTER_OTLP_ENDPOINT=
ENV OTEL_SERVICE_NAME=examples-java

ENTRYPOINT ["java", "-jar", "/app/app.jar"]


