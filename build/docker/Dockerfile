FROM alpine:3.3
COPY kube2msb /bin/

ENTRYPOINT /bin/kube2msb --kube_master_url=${KUBE_MASTER_URL} --auth_token=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token) --msb_url=${MSB_URL}
