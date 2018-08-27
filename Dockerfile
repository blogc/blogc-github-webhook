FROM golang:1.10-alpine3.8
LABEL maintainer "Rafael Martins <rafael@rafaelmartins.eng.br>"

ENV BLOGC_VERSION 0.14.0

COPY . /code

RUN apk add --no-cache --virtual .build-deps \
        make \
        gcc \
        musl-dev \
    && apk --no-cache add \
        bash \
        git \
    && wget https://github.com/blogc/blogc/releases/download/v$BLOGC_VERSION/blogc-$BLOGC_VERSION.tar.bz2 \
    && tar -xvjf blogc-$BLOGC_VERSION.tar.bz2 \
    && rm blogc-$BLOGC_VERSION.tar.bz2 \
    && ( \
        cd blogc-$BLOGC_VERSION \
        && ./configure \
            --prefix /usr \
            --enable-make \
        && make \
        && make install \
    ) \
    && rm -rf blogc-$BLOGC_VERSION \
    && rm -r /usr/share/man \
    && apk del .build-deps \
    && ( \
        cd /code \
        && go build -o /usr/bin/blogc-github-webhook \
    ) \
    && rm -rf /code

VOLUME ["/data"]
EXPOSE 8000

CMD ["/usr/bin/blogc-github-webhook"]
