package data

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s_networking_v1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/kiali/kiali/kubernetes"
)

func CreateEmptyHTTPRoute(name string, namespace string, hosts []string) *k8s_networking_v1beta1.HTTPRoute {
	vs := k8s_networking_v1beta1.HTTPRoute{}
	vs.Name = name
	vs.Namespace = namespace
	for _, host := range hosts {
		vs.Spec.Hostnames = append(vs.Spec.Hostnames, k8s_networking_v1beta1.Hostname(host))
	}
	return &vs
}

func CreateHTTPRoute(name string, namespace string, gateway string, hosts []string) *k8s_networking_v1beta1.HTTPRoute {
	return AddParentRefToHTTPRoute(gateway, namespace, CreateEmptyHTTPRoute(name, namespace, hosts))
}

func AddParentRefToHTTPRoute(name, namespace string, rt *k8s_networking_v1beta1.HTTPRoute) *k8s_networking_v1beta1.HTTPRoute {
	ns := k8s_networking_v1beta1.Namespace(namespace)
	group := k8s_networking_v1beta1.Group(kubernetes.K8sNetworkingGroupVersionV1Beta1.Group)
	kind := k8s_networking_v1beta1.Kind(kubernetes.K8sActualGatewayType)
	rt.Spec.ParentRefs = append(rt.Spec.ParentRefs, k8s_networking_v1beta1.ParentReference{
		Name:      k8s_networking_v1beta1.ObjectName(name),
		Namespace: &ns,
		Group:     &group,
		Kind:      &kind})
	return rt
}

func AddBackendRefToHTTPRoute(name, namespace string, rt *k8s_networking_v1beta1.HTTPRoute) *k8s_networking_v1beta1.HTTPRoute {
	kind := k8s_networking_v1beta1.Kind("Service")
	var ns k8s_networking_v1beta1.Namespace
	if namespace != "" {
		ns = k8s_networking_v1beta1.Namespace(namespace)
	}
	backendRef := k8s_networking_v1beta1.HTTPBackendRef{
		BackendRef: k8s_networking_v1beta1.BackendRef{
			BackendObjectReference: k8s_networking_v1beta1.BackendObjectReference{
				Kind:      &kind,
				Name:      k8s_networking_v1beta1.ObjectName(name),
				Namespace: &ns,
			},
		},
	}
	rule := k8s_networking_v1beta1.HTTPRouteRule{}
	rule.BackendRefs = append(rule.BackendRefs, backendRef)
	rt.Spec.Rules = append(rt.Spec.Rules, rule)
	return rt
}

func CreateEmptyK8sGateway(name, namespace string) *k8s_networking_v1beta1.Gateway {
	gw := k8s_networking_v1beta1.Gateway{}
	gw.Name = name
	gw.Namespace = namespace

	gw.Kind = kubernetes.K8sActualGatewayType
	gw.APIVersion = kubernetes.K8sApiNetworkingVersionV1Beta1
	gw.Spec.GatewayClassName = "istio"
	return &gw
}

func AddListenerToK8sGateway(listener k8s_networking_v1beta1.Listener, gw *k8s_networking_v1beta1.Gateway) *k8s_networking_v1beta1.Gateway {
	gw.Spec.Listeners = append(gw.Spec.Listeners, listener)
	return gw
}

func AddGwAddressToK8sGateway(address k8s_networking_v1beta1.GatewayAddress, gw *k8s_networking_v1beta1.Gateway) *k8s_networking_v1beta1.Gateway {
	gw.Spec.Addresses = append(gw.Spec.Addresses, address)
	return gw
}

func CreateListener(name string, hostname string, port int, protocol string) k8s_networking_v1beta1.Listener {
	hn := k8s_networking_v1beta1.Hostname(hostname)
	listener := k8s_networking_v1beta1.Listener{
		Name:     k8s_networking_v1beta1.SectionName(name),
		Hostname: &hn,
		Port:     k8s_networking_v1beta1.PortNumber(port),
		Protocol: k8s_networking_v1beta1.ProtocolType(protocol),
	}
	return listener
}

func CreateGWAddress(addrType k8s_networking_v1beta1.AddressType, value string) k8s_networking_v1beta1.GatewayAddress {
	address := k8s_networking_v1beta1.GatewayAddress{
		Type:  &addrType,
		Value: value,
	}
	return address
}

func UpdateConditionWithError(k8sgw *k8s_networking_v1beta1.Gateway) *k8s_networking_v1beta1.Gateway {
	condition := metav1.Condition{Type: "Ready", Status: "False", Reason: "", Message: "Fake msg"}
	k8sgw.Status.Conditions = append(k8sgw.Status.Conditions, condition)

	return k8sgw
}
