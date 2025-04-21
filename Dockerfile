FROM golang:latest

ARG ZENA_DIR=/var/lib/zena
ENV ZENA_DIR=$ZENA_DIR

RUN apt-get update -y && apt-get upgrade -y \
    && apt install build-essential git -y \
    && mkdir -p ${ZENA_DIR}

WORKDIR ${ZENA_DIR}
COPY . .
RUN make zena

RUN cp build/bin/zena /usr/bin/

ENV SHELL /bin/bash
EXPOSE 8545 8546 8547 30303 30303/udp

ENTRYPOINT ["zena"]
