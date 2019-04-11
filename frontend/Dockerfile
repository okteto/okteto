FROM node:10 as builder
RUN mkdir /usr/src/app
WORKDIR /usr/src/app
ENV PATH /usr/src/app/node_modules/.bin:$PATH
COPY package.json yarn.lock ./
RUN yarn install
COPY . .
RUN yarn run build

FROM nginx:1.13.9-alpine
COPY --from=builder /usr/src/app/dist /usr/share/nginx/html
COPY nginx.default.conf /etc/nginx/conf.d/default.conf
EXPOSE 8000
CMD ["nginx", "-g", "daemon off;"]