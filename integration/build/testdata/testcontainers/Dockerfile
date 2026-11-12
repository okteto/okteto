# Compiles and runs the test suite, which uses Testcontainers. The Dockerfile doesn't declare
# DOCKER_HOST: the okteto CLI declares it in every stage because of `docker: true` in the manifest.
FROM maven:3.9-eclipse-temurin-21 AS build

WORKDIR /src
COPY pom.xml .
COPY src ./src

RUN mvn -B package

FROM eclipse-temurin:21-jre

WORKDIR /app
COPY --from=build /src/target/testcontainers-example-1.0.0.jar app.jar
COPY --from=build /src/target/lib ./lib

CMD ["java", "-cp", "app.jar:lib/*", "example.HelloWorld"]
