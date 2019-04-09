FROM node:8 as builder
WORKDIR /usr/src/frontend
COPY package.json yarn.lock ./
RUN yarn install
COPY . .
RUN yarn run build

FROM busybox
RUN mkdir /content
COPY --from=builder /pod-data /content