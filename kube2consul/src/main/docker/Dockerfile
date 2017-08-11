FROM alpine:3.3
ENV CONSUL_VERSION 0.7.1
ENV BASE /
ADD consul-linux_amd64.tar.gz /
RUN cd /usr/lib \
  && ln -s /consul/libglib-2.0.so.0.4400.0 libglib-2.0.so.0 \
  && ln -s /consul/libintl.so.8.1.3 libintl.so.8
COPY kube2consul /bin/
COPY start.sh /

ENTRYPOINT exec /start.sh