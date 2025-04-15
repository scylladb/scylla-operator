// Copyright (C) 2025 ScyllaDB

package scylladbmanagerclusterregistration

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
	"github.com/scylladb/scylla-manager/v3/swagger/gen/scylla-manager/models"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/helpers/managerclienterrors"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	hashutil "github.com/scylladb/scylla-operator/pkg/util/hash"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (smcrc *Controller) syncManager(
	ctx context.Context,
	smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration,
	status *scyllav1alpha1.ScyllaDBManagerClusterRegistrationStatus,
) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	var host, authTokenSecretName string
	switch smcr.Spec.ScyllaDBClusterRef.Kind {
	case scyllav1alpha1.ScyllaDBDatacenterGVK.Kind:
		sdc, err := smcrc.scyllaDBDatacenterLister.ScyllaDBDatacenters(smcr.Namespace).Get(smcr.Spec.ScyllaDBClusterRef.Name)
		if err != nil {
			return progressingConditions, fmt.Errorf("can't get ScyllaDBDatacenter %q: %w", naming.ManualRef(smcr.Namespace, smcr.Spec.ScyllaDBClusterRef.Name), err)
		}

		isScyllaDBDatacenterAvailable := sdc.Status.AvailableNodes != nil && *sdc.Status.AvailableNodes > 0
		if !isScyllaDBDatacenterAvailable {
			progressingConditions = append(progressingConditions, metav1.Condition{
				Type:               managerControllerProgressingCondition,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: smcr.Generation,
				Reason:             "AwaitingScyllaDBDatacenterAvailability",
				Message:            fmt.Sprintf("Awaiting ScyllaDBDatacenter %q availability.", naming.ObjRef(sdc)),
			})

			return progressingConditions, nil
		}

		host = naming.CrossNamespaceServiceName(sdc)
		authTokenSecretName = naming.AgentAuthTokenSecretName(sdc)

	default:
		return progressingConditions, fmt.Errorf("unsupported scyllaDBClusterRef Kind: %q", smcr.Spec.ScyllaDBClusterRef.Kind)

	}

	authToken, err := smcrc.getAuthToken(ctx, smcr.Namespace, authTokenSecretName)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't get auth token: %w", err)
	}

	requiredManagerCluster, err := makeRequiredScyllaDBManagerCluster(
		scyllaDBManagerClusterName(smcr),
		string(smcr.UID),
		host,
		authToken,
	)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make required ScyllaDB Manager cluster: %w", err)
	}

	managerClient, err := controllerhelpers.GetScyllaDBManagerClient(ctx, smcr)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't get manager client: %w", err)
	}

	managerCluster, found, err := getScyllaDBManagerCluster(ctx, smcr, managerClient)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't ScyllaDB Manager cluster: %w", err)
	}

	if !found {
		klog.V(4).InfoS("Creating ScyllaDB Manager cluster.", "ScyllaDBManagerClusterRegistration", klog.KObj(smcr), "ScyllaDBManagerClusterName", requiredManagerCluster.Name)

		var managerClusterID string
		managerClusterID, err = managerClient.CreateCluster(ctx, requiredManagerCluster)
		if err != nil {
			klog.V(4).InfoS("Failed to create ScyllaDB Manager cluster", "ScyllaDBManagerClusterRegistration", klog.KObj(smcr), "ScyllaDBManagerClusterName", requiredManagerCluster.Name, "Error", err)
			return progressingConditions, fmt.Errorf("can't create ScyllaDB Manager cluster %q: %s", requiredManagerCluster.Name, managerclienterrors.GetPayloadMessage(err))
		}

		status.ClusterID = &managerClusterID
		progressingConditions = append(progressingConditions, metav1.Condition{
			Type:               managerControllerProgressingCondition,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: smcr.Generation,
			Reason:             "CreatedScyllaDBManagerCluster",
			Message:            fmt.Sprintf("Created a ScyllaDB Manager cluster: %s (%s).", requiredManagerCluster.Name, managerClusterID),
		})
		return progressingConditions, nil
	}

	ownerUIDLabelValue, hasOwnerUIDLabel := managerCluster.Labels[naming.OwnerUIDLabel]
	if !hasOwnerUIDLabel {
		klog.Warningf("ScyllaDB Manager cluster %q is missing the owner UID label. Deleting it to avoid a name collision.", managerCluster.Name)

		err = managerClient.DeleteCluster(ctx, managerCluster.ID)
		if err != nil {
			klog.V(4).InfoS("Failed to delete ScyllaDB Manager cluster", "ScyllaDBManagerClusterRegistration", klog.KObj(smcr), "ScyllaDBManagerClusterName", managerCluster.Name, "ScyllaDBManagerClusterID", managerCluster.ID, "Error", err)
			return progressingConditions, fmt.Errorf("can't delete ScyllaDB Manager cluster %q: %s", managerCluster.Name, managerclienterrors.GetPayloadMessage(err))
		}

		progressingConditions = append(progressingConditions, metav1.Condition{
			Type:               managerControllerProgressingCondition,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: smcr.Generation,
			Reason:             "DeletedCollidingScyllaDBManagerCluster",
			Message:            "Deleted a colliding ScyllaDB Manager cluster with no OwnerUID label.",
		})
		return progressingConditions, nil
	}

	status.ClusterID = &managerCluster.ID

	if ownerUIDLabelValue == string(smcr.UID) && requiredManagerCluster.Labels[naming.ManagedHash] == managerCluster.Labels[naming.ManagedHash] {
		// Cluster matches the desired state, nothing to do.
		return progressingConditions, nil
	}

	if ownerUIDLabelValue != string(smcr.UID) {
		// Ideally we wouldn't do anything here as this is error-prone and might hinder discovering bugs.
		// However, the cluster could have been created by the legacy component (manager-controller), so we update it to become a new owner without disrupting the state.
		klog.Warningf("Cluster %q already exists in ScyllaDB Manager state and has an owner UID label (%q), but it has a different owner. ScyllaDBManagerClusterRegistration %q will adopt it.", managerCluster.Name, ownerUIDLabelValue, klog.KObj(smcr))
	}

	requiredManagerCluster.ID = managerCluster.ID

	klog.V(4).InfoS("Updating ScyllaDB Manager cluster.", "ScyllaDBManagerClusterRegistration", klog.KObj(smcr), "ScyllaDBManagerClusterName", requiredManagerCluster.Name, "ScyllaDBManagerClusterID", requiredManagerCluster.ID)
	err = managerClient.UpdateCluster(ctx, requiredManagerCluster)
	if err != nil {
		klog.V(4).InfoS("Failed to update ScyllaDB Manager cluster", "ScyllaDBManagerClusterRegistration", klog.KObj(smcr), "ScyllaDBManagerClusterName", requiredManagerCluster.Name, "ScyllaDBManagerClusterID", requiredManagerCluster.ID, "Error", err)
		return progressingConditions, fmt.Errorf("can't update ScyllaDB Manager cluster %q: %s", managerCluster.Name, managerclienterrors.GetPayloadMessage(err))
	}

	progressingConditions = append(progressingConditions, metav1.Condition{
		Type:               managerControllerProgressingCondition,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: smcr.Generation,
		Reason:             "UpdatedScyllaDBManagerCluster",
		Message:            fmt.Sprintf("Updated a ScyllaDB Manager cluster: %s (%s).", managerCluster.Name, managerCluster.ID),
	})
	return progressingConditions, nil
}

func (smcrc *Controller) getAuthToken(ctx context.Context, authTokenSecretNamespace, authTokenSecretName string) (string, error) {
	authTokenSecret, err := smcrc.secretLister.Secrets(authTokenSecretNamespace).Get(authTokenSecretName)
	if err != nil {
		return "", fmt.Errorf("can't get secret %q: %w", naming.ManualRef(authTokenSecretNamespace, authTokenSecretName), err)
	}

	authToken, err := helpers.GetAgentAuthTokenFromSecret(authTokenSecret)
	if err != nil {
		return "", fmt.Errorf("can't get agent auth token from secret %q: %w", naming.ObjRef(authTokenSecret), err)
	}

	return authToken, nil
}

func scyllaDBManagerClusterName(smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration) string {
	nameOverrideAnnotationValue, hasNameOverrideAnnotation := smcr.Annotations[naming.ScyllaDBManagerClusterRegistrationNameOverrideAnnotation]
	if hasNameOverrideAnnotation {
		return nameOverrideAnnotationValue
	}

	namespacePrefix := ""
	if smcr.Labels[naming.GlobalScyllaDBManagerLabel] == naming.LabelValueTrue {
		namespacePrefix = smcr.Namespace + "/"
	}

	return namespacePrefix + smcr.Spec.ScyllaDBClusterRef.Kind + "/" + smcr.Spec.ScyllaDBClusterRef.Name
}

func makeRequiredScyllaDBManagerCluster(name, ownerUID, host, authToken string) (*managerclient.Cluster, error) {
	requiredManagerCluster := &managerclient.Cluster{
		Name:      name,
		Host:      host,
		AuthToken: authToken,
		// TODO: enable CQL over TLS when https://github.com/scylladb/scylla-operator/issues/1673 is completed
		ForceNonSslSessionPort: true,
		ForceTLSDisabled:       true,
		Labels: map[string]string{
			naming.OwnerUIDLabel: ownerUID,
		},
		// Disable the creation of the default repair task to avoid name collisions with ScyllaDBManagerTasks.
		WithoutRepair: true,
	}

	managedHash, err := hashutil.HashObjects(requiredManagerCluster)
	if err != nil {
		return nil, fmt.Errorf("can't calculate managed hash: %w", err)
	}
	requiredManagerCluster.Labels[naming.ManagedHash] = managedHash

	return requiredManagerCluster, nil
}

func getScyllaDBManagerCluster(ctx context.Context, smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration, managerClient *managerclient.Client) (*managerclient.Cluster, bool, error) {
	managerClusterName := scyllaDBManagerClusterName(smcr)

	if smcr.Status.ClusterID != nil {
		managerCluster, err := managerClient.GetCluster(ctx, *smcr.Status.ClusterID)
		if err != nil {
			if !managerclienterrors.IsNotFound(err) {
				klog.V(4).InfoS("Failed to get ScyllaDB Manager cluster by ID", "ScyllaDBManagerClusterRegistration", klog.KObj(smcr), "ScyllaDBManagerClusterID", *smcr.Status.ClusterID, "Error", err)
				return nil, false, fmt.Errorf("can't get ScyllaDB Manager cluster: %s", managerclienterrors.GetPayloadMessage(err))
			}

			klog.Warningf("Cluster %q (%q) owned by ScyllaDBManagerClusterRegistration %q has been removed from ScyllaDB Manager state.", managerClusterName, *smcr.Status.ClusterID, klog.KObj(smcr))
			// Fall back to getting by name.
		} else {
			return managerCluster, true, nil
		}
	}

	// TODO: get a single cluster instead when https://github.com/scylladb/scylla-manager/issues/4350 is implemented.
	managerClusters, err := managerClient.ListClusters(ctx)
	if err != nil {
		klog.V(4).InfoS("Failed to list ScyllaDB Manager clusters", "ScyllaDBManagerClusterRegistration", klog.KObj(smcr), "Error", err)
		return nil, false, fmt.Errorf("can't list ScyllaDB Manager clusters: %s", managerclienterrors.GetPayloadMessage(err))
	}

	// Cluster names in manager state are unique, so it suffices to only find one with a matching name.
	managerCluster, _, found := oslices.Find(managerClusters, func(c *models.Cluster) bool {
		return c.Name == managerClusterName
	})

	return managerCluster, found, nil
}
