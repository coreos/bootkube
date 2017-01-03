#!/bin/sh
MIRROR=${MIRROR:-{{ MIRROR }}}
sed -i "s#{{ MIRROR }}#${MIRROR}#" /checkpoint-pod.yaml
cp /checkpoint-pod.yaml /etc/kubernetes/manifests
while true
do
  sleep 3600
done
