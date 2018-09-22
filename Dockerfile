FROM golang:1.11-stretch
LABEL maintainer "Rafael Martins <rafael@rafaelmartins.eng.br>"

ENV BLOGC_VERSION 0.14.0

COPY . /code

RUN set -x \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        wget \
        git \
        tar \
        xz-utils \
        locales-all \
    && rm -rf /var/lib/apt/lists/* \
    && wget https://github.com/blogc/blogc/releases/download/v$BLOGC_VERSION/blogc-$BLOGC_VERSION.tar.xz \
    && tar -xvf blogc-$BLOGC_VERSION.tar.xz \
    && rm blogc-$BLOGC_VERSION.tar.xz \
    && ( \
        cd blogc-$BLOGC_VERSION \
        && ./configure \
            --prefix /usr \
            --enable-make \
        && make \
        && make install \
    ) \
    && rm -rf blogc-$BLOGC_VERSION \
    && ( \
        cd /code \
        && go build -o /usr/bin/blogc-github-webhook \
    ) \
    && rm -rf /code

ENV BGW_BASEDIR /data

VOLUME ["/data"]
EXPOSE 8000

CMD ["/usr/bin/blogc-github-webhook"]
