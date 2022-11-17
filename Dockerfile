FROM golang:1.18.4-alpine3.15 as builder
MAINTAINER chaoyue
ARG PROJECT
ARG BIN_LABELS
ENV GOPROXY https://goproxy.cn/
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apk/repositories
RUN apk update && apk add tzdata make git
WORKDIR /${PROJECT}
COPY . /${PROJECT}
COPY [^.]* /${PROJECT} 
RUN make build-lux

FROM houchaoyue/alpine_base:latest
ARG PROJECT
ARG BIN_LABELS
ENV ENV_PROJECT=${PROJECT}
ENV ENV_BIN_LABELS=${BIN_LABELS}
COPY --from=builder /${PROJECT}/${BIN_LABELS} /${PROJECT}/${BIN_LABELS}
COPY --from=builder /usr/share/zoneinfo/Asia/Shanghai /etc/localtime
EXPOSE 8000
WORKDIR /${PROJECT}
CMD [ "/${ENV_PROJECT}/${ENV_BIN_LABELS}" ]