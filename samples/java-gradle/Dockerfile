FROM gradle:6.5 AS build
WORKDIR /code
COPY . /code/
RUN gradle build

FROM openjdk:8-jre
EXPOSE 8080
WORKDIR /app
COPY --from=build /code/build/libs/*.jar .
CMD java -jar *.jar
