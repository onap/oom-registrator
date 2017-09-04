FROM alpine:3.3
COPY kube2msb /bin/

ENTRYPOINT /bin/kube2msb  --kube_master_url=${KUBE_MASTER_URL} --msb_url=${MSB_URL}