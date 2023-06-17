#!/bin/bash

# This deploys the error rates demo

HACK_SCRIPT_DIR="$(cd $(dirname "${BASH_SOURCE[0]}") && pwd)"
source ${HACK_SCRIPT_DIR}/functions.sh


: ${CLIENT_EXE:=oc}
: ${ARCH:=amd64}
: ${DELETE_DEMO:=false}
: ${ENABLE_INJECTION:=true}
: ${ISTIO_NAMESPACE:=istio-system}
: ${NAMESPACE_ALPHA:=alpha}
: ${NAMESPACE_BETA:=beta}
: ${SOURCE:="https://raw.githubusercontent.com/kiali/demos/master"}
: ${DISTRIBUTE_DEMO:=false}
: ${CLUSTER1_CONTEXT=east}
: ${CLUSTER2_CONTEXT=west}

while [ $# -gt 0 ]; do
  key="$1"
  case $key in
    -a|--arch)
      ARCH="$2"
      shift;shift
      ;;
    -c|--client)
      CLIENT_EXE="$2"
      shift;shift
      ;;
    -d|--delete)
      DELETE_DEMO="$2"
      shift;shift
      ;;
    -ei|--enable-injection)
      ENABLE_INJECTION="$2"
      shift;shift
      ;;
    -in|--istio-namespace)
      ISTIO_NAMESPACE="$2"
      shift;shift
      ;;
    -dd|--distribute-demo)
      DISTRIBUTE_DEMO="$2"
      shift;shift
      ;;
    -h|--help)
      cat <<HELPMSG
Valid command line arguments:
  -a|--arch <amd64|ppc64le|s390x>: Images for given arch will be used (default: amd64).
  -c|--client: either 'oc' or 'kubectl'
  -d|--delete: either 'true' or 'false'. If 'true' the demo will be deleted, not installed.
  -ei|--enable-injection: either 'true' or 'false' (default is true). If 'true' auto-inject proxies for the workloads.
  -in|--istio-namespace <name>: Where the Istio control plane is installed (default: istio-system).
  -dd|--distribute-demo 'true' or 'false'. If 'true' alpha namespace will be created on east cluster and beta namespace on west cluster.
  -h|--help: this text
  -s|--source: demo file source. For example: file:///home/me/demos Default: https://raw.githubusercontent.com/kiali/demos/master
HELPMSG
      exit 1
      ;;
    -s|--source)
      SOURCE="$2"
      shift;shift
      ;;
    *)
      echo "Unknown argument [$key]. Aborting."
      exit 1
      ;;
  esac
done

echo Will deploy Error Rates Demo using these settings:
echo CLIENT_EXE=${CLIENT_EXE}
echo ARCH=${ARCH}
echo DELETE_DEMO=${DELETE_DEMO}
echo ENABLE_INJECTION=${ENABLE_INJECTION}
echo ISTIO_NAMESPACE=${ISTIO_NAMESPACE}
echo NAMESPACE_ALPHA=${NAMESPACE_ALPHA}
echo NAMESPACE_BETA=${NAMESPACE_BETA}
echo SOURCE=${SOURCE}

# check arch values
if [ "${ARCH}" != "ppc64le" ] && [ "${ARCH}" != "s390x" ] && [ "${ARCH}" != "amd64" ]; then
  echo "${ARCH} is not supported. Exiting."
  exit 1
fi

IS_OPENSHIFT="false"
IS_MAISTRA="false"
if [[ "${CLIENT_EXE}" = *"oc" ]]; then
  IS_OPENSHIFT="true"
  IS_MAISTRA=$([ "$(oc get crd | grep servicemesh | wc -l)" -gt "0" ] && echo "true" || echo "false")
fi

if [ "${IS_OPENSHIFT}" == "true" ] && [ "${DISTRIBUTE_DEMO}" == "true" ]; then
  echo "Distribute demo is not supported on OpenShift. Exiting."
  exit 1
fi

echo "IS_OPENSHIFT=${IS_OPENSHIFT}"
echo "IS_MAISTRA=${IS_MAISTRA}"

# If we are to delete, remove everything and exit immediately after
if [ "${DELETE_DEMO}" == "true" ]; then
  echo "Deleting Error Rates Demo (the envoy filters, if previously created, will remain)"
  if [ "${IS_OPENSHIFT}" == "true" ]; then
    if [ "${IS_MAISTRA}" != "true" ]; then
      $CLIENT_EXE delete network-attachment-definition istio-cni -n ${NAMESPACE_ALPHA}
      $CLIENT_EXE delete network-attachment-definition istio-cni -n ${NAMESPACE_BETA}
    else
      $CLIENT_EXE delete smm default -n ${NAMESPACE_ALPHA}
      $CLIENT_EXE delete smm default -n ${NAMESPACE_BETA}
    fi
    $CLIENT_EXE delete scc error-rates-scc
  fi
  
  if [ "${DISTRIBUTE_DEMO}" == "true" ]; then
    ${CLIENT_EXE} delete namespace ${NAMESPACE_ALPHA} --context ${CLUSTER1_CONTEXT}
    ${CLIENT_EXE} delete namespace ${NAMESPACE_BETA}  --context ${CLUSTER2_CONTEXT}
  else
    ${CLIENT_EXE} delete namespace ${NAMESPACE_ALPHA}
    ${CLIENT_EXE} delete namespace ${NAMESPACE_BETA}
  fi
  
  exit 0
fi

# Create and prepare the demo namespaces

if [ "${IS_OPENSHIFT}" == "true" ]; then
  $CLIENT_EXE new-project ${NAMESPACE_ALPHA}
  $CLIENT_EXE new-project ${NAMESPACE_BETA}
else
  if [ "${DISTRIBUTE_DEMO}" == "true" ]; then
    $CLIENT_EXE create namespace ${NAMESPACE_ALPHA} --context ${CLUSTER1_CONTEXT}
    $CLIENT_EXE create namespace ${NAMESPACE_BETA} --context ${CLUSTER2_CONTEXT}
  else
    $CLIENT_EXE create namespace ${NAMESPACE_ALPHA}
    $CLIENT_EXE create namespace ${NAMESPACE_BETA}
  fi
fi

if [ "${ENABLE_INJECTION}" == "true" ]; then
 if [ "${DISTRIBUTE_DEMO}" == "true" ]; then
    ${CLIENT_EXE} label namespace ${NAMESPACE_ALPHA} istio-injection=enabled --context ${CLUSTER1_CONTEXT}
    ${CLIENT_EXE} label namespace ${NAMESPACE_BETA} istio-injection=enabled  --context ${CLUSTER2_CONTEXT}
  else
    ${CLIENT_EXE} label namespace ${NAMESPACE_ALPHA} istio-injection=enabled
    ${CLIENT_EXE} label namespace ${NAMESPACE_BETA} istio-injection=enabled
  fi
fi

# For OpenShift 4.11, adds default service account in the current ns to use as a user
if [ "${IS_OPENSHIFT}" == "true" ]; then
  $CLIENT_EXE adm policy add-scc-to-user anyuid -z default -n ${NAMESPACE_ALPHA}
  $CLIENT_EXE adm policy add-scc-to-user anyuid -z default -n ${NAMESPACE_BETA}
fi

if [ "${IS_OPENSHIFT}" == "true" ]; then
  if [ "${IS_MAISTRA}" != "true" ]; then
    cat <<NAD | $CLIENT_EXE -n ${NAMESPACE_ALPHA} create -f -
apiVersion: "k8s.cni.cncf.io/v1"
kind: NetworkAttachmentDefinition
metadata:
  name: istio-cni
NAD
    cat <<NAD | $CLIENT_EXE -n ${NAMESPACE_BETA} create -f -
apiVersion: "k8s.cni.cncf.io/v1"
kind: NetworkAttachmentDefinition
metadata:
  name: istio-cni
NAD
  fi
  cat <<SCC | $CLIENT_EXE apply -f -
apiVersion: security.openshift.io/v1
kind: SecurityContextConstraints
metadata:
  name: error-rates-scc
runAsUser:
  type: RunAsAny
seLinuxContext:
  type: RunAsAny
supplementalGroups:
  type: RunAsAny
users:
- "system:serviceaccount:${NAMESPACE_ALPHA}:default"
- "system:serviceaccount:${NAMESPACE_BETA}:default"
SCC
fi

# Deploy the demo
url_alpha="${SOURCE}/error-rates/alpha.yaml"
url_beta="${SOURCE}/error-rates/beta.yaml"
sed_client_p="s;kiali/demo_error_rates_client;maistra/demo_error_rates_client-p;g"
sed_server_p="s;kiali/demo_error_rates_server;maistra/demo_error_rates_server-p;g"
sed_client_z="s;kiali/demo_error_rates_client;maistra/demo_error_rates_client-z;g"
sed_server_z="s;kiali/demo_error_rates_server;maistra/demo_error_rates_server-z;g"

if [ "${DISTRIBUTE_DEMO}" == "true" ]; then
  if [ "${ARCH}" == "ppc64le" ]; then
    ${CLIENT_EXE} apply -f <(curl -L ${url_alpha} | sed "${sed_client_p}" | sed "${sed_server_p}") -n ${NAMESPACE_ALPHA} --context ${CLUSTER1_CONTEXT}
    ${CLIENT_EXE} apply -f <(curl -L "${url_beta}" | sed "${sed_client_p}" | sed "${sed_server_p}") -n ${NAMESPACE_BETA} --context ${CLUSTER2_CONTEXT}
  elif [ "${ARCH}" == "s390x" ]; then
    ${CLIENT_EXE} apply -f <(curl -L ${url_alpha} | sed "${sed_client_z}" | sed "${sed_server_z}") -n ${NAMESPACE_ALPHA} --context ${CLUSTER1_CONTEXT}
    ${CLIENT_EXE} apply -f <(curl -L "${url_beta}" | sed "${sed_client_z}" | sed "${sed_server_z}") -n ${NAMESPACE_BETA} --context ${CLUSTER2_CONTEXT}
  else
    ${CLIENT_EXE} apply -f <(curl -L ${url_alpha}) -n ${NAMESPACE_ALPHA} --context ${CLUSTER1_CONTEXT}
    ${CLIENT_EXE} apply -f <(curl -L "${url_beta}") -n ${NAMESPACE_BETA} --context ${CLUSTER2_CONTEXT}
  fi
else
  if [ "${ARCH}" == "ppc64le" ]; then
    ${CLIENT_EXE} apply -f <(curl -L ${url_alpha} | sed "${sed_client_p}" | sed "${sed_server_p}") -n ${NAMESPACE_ALPHA}
    ${CLIENT_EXE} apply -f <(curl -L "${url_beta}" | sed "${sed_client_p}" | sed "${sed_server_p}") -n ${NAMESPACE_BETA}
  elif [ "${ARCH}" == "s390x" ]; then
    ${CLIENT_EXE} apply -f <(curl -L ${url_alpha} | sed "${sed_client_z}" | sed "${sed_server_z}") -n ${NAMESPACE_ALPHA}
    ${CLIENT_EXE} apply -f <(curl -L "${url_beta}" | sed "${sed_client_z}" | sed "${sed_server_z}") -n ${NAMESPACE_BETA}
  else
    ${CLIENT_EXE} apply -f <(curl -L ${url_alpha}) -n ${NAMESPACE_ALPHA}
    ${CLIENT_EXE} apply -f <(curl -L "${url_beta}") -n ${NAMESPACE_BETA}
  fi
fi

# we need to update deployment annotations after we create it
if [ "${IS_MAISTRA}" == "true" ]; then
  prepare_maistra "${NAMESPACE_ALPHA}"
  prepare_maistra "${NAMESPACE_BETA}"
fi
