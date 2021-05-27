// Copyright (C) 2021 ScyllaDB

package resourceapply_test

import (
	"context"
	"testing"

	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/util/resourceapply"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestApplySecret(t *testing.T) {
	ctx := context.Background()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: make(map[string]string),
			Name:        "secret-name",
			Namespace:   "secret-namespace",
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"key": []byte("value"),
		},
	}

	secretWithHash := secret.DeepCopy()
	secretWithHash.Annotations[naming.ManagedHash] = resourceapply.MustHashObjects(secretWithHash)

	differentSecret := secret.DeepCopy()
	differentSecret.Data = map[string][]byte{
		"yek": []byte("eulav"),
	}

	tests := []struct {
		Name         string
		Objects      []runtime.Object
		Secret       *corev1.Secret
		ExpectedHash string
		ExpectEvent  bool
	}{
		{
			Name:         "object is created when it doesn't exists",
			Objects:      []runtime.Object{},
			Secret:       secret.DeepCopy(),
			ExpectedHash: resourceapply.MustHashObjects(secret.DeepCopy()),
			ExpectEvent:  true,
		},
		{
			Name:         "object isn't updated when it has the same hash",
			Objects:      []runtime.Object{secretWithHash.DeepCopy()},
			Secret:       secret.DeepCopy(),
			ExpectedHash: secretWithHash.Annotations[naming.ManagedHash],
			ExpectEvent:  false,
		},
		{
			Name:         "object is updated when it is different than existing one",
			Objects:      []runtime.Object{secretWithHash.DeepCopy()},
			Secret:       differentSecret.DeepCopy(),
			ExpectedHash: resourceapply.MustHashObjects(differentSecret.DeepCopy()),
			ExpectEvent:  true,
		},
	}

	for i := range tests {
		test := tests[i]
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			client := fake.NewClientBuilder().
				WithScheme(ClientGoScheme()).
				WithRuntimeObjects(test.Objects...).
				Build()
			recorder := record.NewFakeRecorder(10)

			if err := resourceapply.ApplySecret(ctx, recorder, client, test.Secret); err != nil {
				t.Errorf("failed to apply secret, got err %v", err)
			}

			secretKey := naming.NamespacedName(test.Secret.Name, test.Secret.Namespace)
			result := &corev1.Secret{}
			if err := client.Get(ctx, secretKey, result); err != nil {
				t.Errorf("failed to get secret, got err %v", err)
			}

			if result.Annotations[naming.ManagedHash] != test.ExpectedHash {
				t.Errorf("object hash is different then expected %s", result.Annotations[naming.ManagedHash])
			}

			if test.ExpectEvent {
				select {
				case _ = <-recorder.Events:
				default:
					t.Errorf("expected event but it wasn't emitted")
				}
			} else {
				if len(recorder.Events) != 0 {
					t.Errorf("expected no event but it was emitted, %s", <-recorder.Events)
				}
			}
		})
	}
}
