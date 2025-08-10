FROM maven:3.9.6-eclipse-temurin-17 as build
WORKDIR /workspace
COPY pom.xml .
COPY src ./src
RUN mvn -q -e -DskipTests package

FROM eclipse-temurin:17-jre
WORKDIR /app
COPY --from=build /workspace/target/java-sample-1.0-SNAPSHOT.jar /app/app.jar

EXPOSE 8082

ENV OTEL_EXPORTER_OTLP_ENDPOINT=
ENV OTEL_SERVICE_NAME=examples-java

ENTRYPOINT ["java", "-jar", "/app/app.jar"]


