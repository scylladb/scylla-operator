package controllerhelpers

import (
	"fmt"
	"maps"

	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	osnaming "github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ConvertEndpointSlicesToEndpoints(endpointSlices []*discoveryv1.EndpointSlice) ([]*corev1.Endpoints, error) {
	serviceToEndpointSlices := map[string][]*discoveryv1.EndpointSlice{}
	for _, es := range endpointSlices {
		svcName, ok := es.Labels[discoveryv1.LabelServiceName]
		if !ok {
			return nil, fmt.Errorf("EndpointSlice %q is missing service name label", osnaming.ObjRef(es))
		}
		serviceToEndpointSlices[svcName] = append(serviceToEndpointSlices[svcName], es)
	}

	var endpoints []*corev1.Endpoints
	for svcName, ess := range serviceToEndpointSlices {
		var subsets []corev1.EndpointSubset
		for _, es := range ess {
			var addresses []corev1.EndpointAddress
			var notReadyAddresses []corev1.EndpointAddress

			for _, ep := range es.Endpoints {
				for _, epa := range ep.Addresses {
					ea := corev1.EndpointAddress{
						IP: epa,
						Hostname: func() string {
							if ep.Hostname != nil {
								return *ep.Hostname
							}
							return ""
						}(),
						NodeName:  ep.NodeName,
						TargetRef: ep.TargetRef,
					}
					if ep.Conditions.Ready != nil && *ep.Conditions.Ready {
						addresses = append(addresses, ea)
					} else {
						notReadyAddresses = append(notReadyAddresses, ea)
					}
				}
			}

			ports := oslices.ConvertSlice[corev1.EndpointPort, discoveryv1.EndpointPort](es.Ports, func(port discoveryv1.EndpointPort) corev1.EndpointPort {
				ep := corev1.EndpointPort{
					AppProtocol: port.AppProtocol,
				}
				if port.Name != nil {
					ep.Name = *port.Name
				}
				if port.Port != nil {
					ep.Port = *port.Port
				}
				if port.Protocol != nil {
					ep.Protocol = *port.Protocol
				}
				return ep
			})

			if len(addresses) == 0 && len(notReadyAddresses) == 0 {
				continue
			}

			subsets = append(subsets, corev1.EndpointSubset{
				Addresses:         addresses,
				NotReadyAddresses: notReadyAddresses,
				Ports:             ports,
			})
		}

		// Copy metadata from first EndpointSlice of given Service, they should all be the same.
		es := ess[0]
		endpointsLabels := map[string]string{}
		maps.Copy(endpointsLabels, es.Labels)
		endpointsLabels[discoveryv1.LabelSkipMirror] = "true"

		endpointsAnnotations := maps.Clone(es.Annotations)

		endpoints = append(endpoints, &corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{
				Name:            svcName,
				Namespace:       es.Namespace,
				Labels:          endpointsLabels,
				Annotations:     endpointsAnnotations,
				OwnerReferences: es.OwnerReferences,
			},
			Subsets: subsets,
		})
	}

	return endpoints, nil
}
