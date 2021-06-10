package helpers

import (
	"fmt"

	"github.com/scylladb/go-log"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/errors"
)

func GetScyllaIPFromService(svc *corev1.Service) (string, error) {
	if svc.Spec.Type != corev1.ServiceTypeClusterIP {
		return "", fmt.Errorf("service %s is of type %q instead of %q", naming.ObjRef(svc), svc.Spec.Type, corev1.ServiceTypeClusterIP)
	}

	if svc.Spec.ClusterIP == corev1.ClusterIPNone {
		return "", fmt.Errorf("service %s doesn't have a ClusterIP", naming.ObjRef(svc))
	}

	return svc.Spec.ClusterIP, nil
}

func GetScyllaHost(statefulsetName string, ordinal int32, services map[string]*corev1.Service) (string, error) {
	svcName := fmt.Sprintf("%s-%d", statefulsetName, ordinal)
	svc, found := services[svcName]
	if !found {
		return "", fmt.Errorf("missing service %q", svcName)
	}

	ip, err := GetScyllaIPFromService(svc)
	if err != nil {
		return "", err
	}

	return ip, nil
}

func GetRequiredScyllaHosts(sc *scyllav1.ScyllaCluster, services map[string]*corev1.Service) ([]string, error) {
	var hosts []string
	var errs []error
	for _, rack := range sc.Spec.Datacenter.Racks {
		for ord := int32(0); ord < rack.Members; ord++ {
			stsName := naming.StatefulSetNameForRack(rack, sc)
			host, err := GetScyllaHost(stsName, ord, services)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			hosts = append(hosts, host)
		}
	}
	var err error = errors.NewAggregate(errs)
	if err != nil {
		return nil, err
	}

	return hosts, nil
}

func NewScyllaClientFromToken(hosts []string, authToken string) (*scyllaclient.Client, error) {
	// TODO: unify logging
	logger, _ := log.NewProduction(log.Config{
		Level: zap.NewAtomicLevelAt(zapcore.InfoLevel),
	})

	cfg := scyllaclient.DefaultConfig(authToken, hosts...)
	scyllaClient, err := scyllaclient.NewClient(cfg, logger.Named("scylla_client"))
	if err != nil {
		return nil, err
	}

	return scyllaClient, nil
}

func NewScyllaClientFromSecret(secret *corev1.Secret, hosts []string) (*scyllaclient.Client, error) {
	token, err := helpers.GetAgentAuthTokenFromSecret(secret)
	if err != nil {
		return nil, err
	}

	return NewScyllaClientFromToken(hosts, token)
}

func SetRackCondition(rackStatus *scyllav1.RackStatus, newCondition scyllav1.RackConditionType) {
	for i := range rackStatus.Conditions {
		if rackStatus.Conditions[i].Type == newCondition {
			rackStatus.Conditions[i].Status = corev1.ConditionTrue
			return
		}
	}
	rackStatus.Conditions = append(
		rackStatus.Conditions,
		scyllav1.RackCondition{Type: newCondition, Status: corev1.ConditionTrue},
	)
}
