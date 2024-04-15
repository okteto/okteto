FROM ruby:2 AS builder

WORKDIR /opt/app
COPY Gemfile Gemfile.lock /opt/app/
RUN bundle config set frozen 'true'
RUN bundle install

#######################################

FROM builder AS dev

COPY bashrc /root/.bashrc

ENV APP_ENV development
ENV RUBYOPT "-W:no-deprecated"

RUN bundle config set with 'development'
RUN bundle install

ENV PORT 8080
EXPOSE 8080

#######################################

FROM ruby:2 AS production

WORKDIR /opt/app
COPY --from=builder /usr/local/ /usr/local/
COPY --from=builder /opt/app/ /opt/app
COPY . /opt/app/

ENV PORT 8080
EXPOSE 8080

ENV APP_ENV production
ENV RUBYOPT "-W:no-deprecated"

CMD ["ruby", "./app.rb"]
