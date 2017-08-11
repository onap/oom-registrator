#!/bin/sh
if [ -z "${KUBE_MASTER_IP}" ]; then
        echo "kube master node ip is required."
        exit 1
fi

if [ -n "${JOIN_IP}" ]; then
        echo "### Starting consul client"
		if [ -z "${ALL_IN_ONE}" ]; then
			/consul/consul agent -data-dir /consul-works/data-dir -node kube2consul_${KUBE_MASTER_IP} -bind ${KUBE_MASTER_IP} -client 0.0.0.0 -retry-join ${JOIN_IP} -retry-interval 5s &
		else
			/consul/consul agent -data-dir /consul-works/data-dir -node kube2consul_${KUBE_MASTER_IP} -bind 0.0.0.0 -client 0.0.0.0 -retry-join ${JOIN_IP} -retry-interval 5s &
		fi	
fi

if [ -z "${RUN_MODE}" ]; then
	echo "non-HA scenario."
else
    echo "\n\n### Starting consul agent"
	cd ./consul
	./entry.sh &
fi

kube_url="http://${KUBE_MASTER_IP}:8080"

if [ "${CLUSTER_TYPE}" == "openshift" ]; then
        kube_url="https://${KUBE_MASTER_IP}:8443"
fi

echo "\n\n### Starting kube2consul"
if [ -z "${PDM_CONTROLLER_IP}" ]; then
        /bin/kube2consul --kube_master_url ${kube_url}
else
        echo "in Paas mode."
        /bin/kube2consul --kube_master_url ${kube_url} --pdm_controller_url http://${PDM_CONTROLLER_IP}:9527
fi