FROM alpine:3.11
COPY kube2msb /bin/

RUN addgroup -g 1000 msb && \
    adduser -D -u 1000 -G msb msb && \
    chown msb:msb /bin/kube2msb
USER msb

ENTRYPOINT /bin/kube2msb --kube_master_url=${KUBE_MASTER_URL} --auth_token=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token) --msb_url=${MSB_URL}
