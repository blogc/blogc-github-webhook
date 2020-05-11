FROM golang:1.14-buster
LABEL maintainer "Rafael Martins <rafael@rafaelmartins.eng.br>"

ENV BGW_BASEDIR /data

COPY . /code

RUN set -x \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        wget \
        tar \
        xz-utils \
        locales-all \
    && rm -rf /var/lib/apt/lists/* \
    && BLOGC_VERSION=$(wget -q -O- https://blogc.rgm.io/ | grep '^LATEST_RELEASE=' | cut -d= -f2) \
    && wget https://github.com/blogc/blogc/releases/download/v$BLOGC_VERSION/blogc-$BLOGC_VERSION.tar.xz \
    && tar -xvf blogc-$BLOGC_VERSION.tar.xz \
    && rm blogc-$BLOGC_VERSION.tar.xz \
    && ( \
        cd blogc-$BLOGC_VERSION \
        && ./configure \
            --prefix /usr/local \
            --enable-make \
        && make \
        && make install \
    ) \
    && rm -rf blogc-$BLOGC_VERSION \
    && ( \
        cd /code \
        && CGO_ENABLED=0 go build -o /usr/local/bin/blogc-github-webhook \
    ) \
    && rm -rf /code

VOLUME /data

EXPOSE 8000

CMD ["/usr/local/bin/blogc-github-webhook"]
