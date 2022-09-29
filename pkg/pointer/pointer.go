package pointer

import (
	"crypto/x509"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func KeyUsage(u x509.KeyUsage) *x509.KeyUsage {
	return &u
}

func Time(t metav1.Time) *metav1.Time {
	return &t
}
