#!/bin/bash

CLUSTER_PREFIX=etcd
CLUSTER_SIZE=1
MEM=1024

# prefer local build if in a git checkout
if [[ -f ../../bin/corectl ]]; then
	CORECTL=../../bin/corectl
else
	CORECTL=$(which corectl)
	if ! [[ $? ]]; then
		echo "${CORECTL} not found (neither in ../../bin/ nor in PATH)"
		exit 1
	fi
fi
echo "using ${CORECTL}"
echo

while [ "$1" != "" ]; do
    case $1 in
        -p | --prefix )
			shift
        	CLUSTER_PREFIX=$1
        ;;
        -s | --size )
			shift
			CLUSTER_SIZE=$1
			if ! [[ "$1" =~ ^[0-9]+$ ]]; then
	            echo "error: cluster size implies integers only..."
				exit -1
			fi
			if [[ "$((${CLUSTER_SIZE}%2))" -eq 0 ]]; then
				echo "error: cluster size odd please..."
				exit -1
			fi
        ;;
        -h | --help )
			echo "$0 -p|--prefix ETCD_CLUSTER_PREFIX -s|--size #ETCD_CLUSTER_SIZE"
        	exit
        ;;
        * )
			$0 -h
        	exit 1
    esac
    shift
done

echo "cluster size is ${CLUSTER_SIZE}"
echo "cluster prefix is ${CLUSTER_PREFIX}"
echo

for (( i=1; i<=${CLUSTER_SIZE}; i++ )); do
   ETCD_CLUSTER[$i]=${CLUSTER_PREFIX}-$i
done

for (( i=1; i<=${CLUSTER_SIZE}; i++ )); do
	ETCD_NODE_NAME=${ETCD_CLUSTER[$i]}
	echo "booting ${ETCD_NODE_NAME}"
	ETCD_INITIAL_CLUSTER=""
	for (( j=1; j<=${CLUSTER_SIZE}; j++ ));	do
		ETCD_INITIAL_CLUSTER=${ETCD_CLUSTER[$j]}=http://${ETCD_CLUSTER[$j]}:2380,${ETCD_INITIAL_CLUSTER}
	done
	ETCD_INITIAL_CLUSTER=$(echo ${ETCD_INITIAL_CLUSTER} | /usr/bin/sed -e "s#,\$##")
	VM_CCONFIG=$(/usr/bin/mktemp)
	/usr/bin/sed -e 's#{{ETCD_NODE_NAME}}#'"${ETCD_NODE_NAME}"'#g' \
		-e 's#{{ETCD_INITIAL_CLUSTER}}#'"${ETCD_INITIAL_CLUSTER}"'#g' \
			etcd-cloud-config.yaml> ${VM_CCONFIG}
	${CORECTL} run -m ${MEM} -n ${ETCD_CLUSTER[$i]} -L ${VM_CCONFIG}
	rm -rf ${VM_CCONFIG}
done
sleep 2
${CORECTL} ssh  ${ETCD_CLUSTER["1"]} "etcdctl cluster-health"
