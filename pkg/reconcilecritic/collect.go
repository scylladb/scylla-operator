// Copyright (c) 2026 ScyllaDB.

package reconcilecritic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// TracesDirName is the directory of the artifacts holding the reconcile traces.
const TracesDirName = "reconcile-traces"

// TraceMeta describes where a trace comes from. It is written next to the traces as <name>.meta.json.
type TraceMeta struct {
	// TestName is the full text of the test that ran while the trace was recorded.
	TestName string `json:"testName"`
	// Failed tells a test that failed.
	Failed bool `json:"failed"`
	// Namespace is the namespace the trace was recorded for, in e2e.
	Namespace string `json:"namespace,omitempty"`
	// Source is "e2e" or "envtest".
	Source          string `json:"source"`
	ParallelProcess int    `json:"parallelProcess,omitempty"`
}

// WriteTraceMeta writes meta to path.
func WriteTraceMeta(path string, meta TraceMeta) error {
	b, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, b, 0o644)
}

// ReadTraceMeta reads the meta of the trace at tracePath: <stem>.meta.json, or, for the per-Pod traces of e2e named
// <namespace>.<pod>.jsonl, <namespace>.meta.json.
func ReadTraceMeta(tracePath string) (*TraceMeta, error) {
	stem := strings.TrimSuffix(tracePath, ".jsonl")
	candidates := []string{stem + ".meta.json"}
	if dir, base := filepath.Split(stem); strings.Contains(base, ".") {
		candidates = append(candidates, filepath.Join(dir, base[:strings.Index(base, ".")]+".meta.json"))
	}

	for _, c := range candidates {
		b, err := os.ReadFile(c)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		meta := &TraceMeta{}
		err = json.Unmarshal(b, meta)
		if err != nil {
			return nil, fmt.Errorf("can't parse %q: %w", c, err)
		}
		return meta, nil
	}

	return nil, nil
}

// CollectOptions say where the operator serves its traces.
type CollectOptions struct {
	OperatorNamespace string
	// OperatorSelector selects the operator Pods.
	OperatorSelector string
	// Port is the port the operator serves the traces on.
	Port int
	// Timeout bounds the fetch from one Pod. Defaults to a minute.
	Timeout time.Duration
}

// FetchTrace fetches the trace of namespace from an operator Pod through the API server's Pod proxy. It returns
// false if the Pod has no trace of the namespace, e.g. because it isn't the leader.
func FetchTrace(ctx context.Context, proxy rest.Interface, operatorNamespace, pod string, port int, namespace string, timeout time.Duration) ([]byte, bool, error) {
	if timeout <= 0 {
		timeout = time.Minute
	}

	body, err := proxy.Get().
		Namespace(operatorNamespace).
		Resource("pods").
		Name(fmt.Sprintf("%s:%d", pod, port)).
		SubResource("proxy").
		Suffix("reconcile-traces", namespace).
		Timeout(timeout).
		DoRaw(ctx)
	if err != nil {
		var statusErr apierrors.APIStatus
		if errors.As(err, &statusErr) && statusErr.Status().Code == http.StatusNotFound {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("can't fetch the reconcile trace of namespace %q from Pod %s/%s: %w", namespace, operatorNamespace, pod, err)
	}

	return body, true, nil
}

// CollectTraces fetches the trace of namespace from every running operator Pod and writes it to
// <dir>/<namespace>.<pod>.jsonl, with meta in <dir>/<namespace>.meta.json. A trace from each Pod stays in a file of
// its own: record IDs are only unique within one process. It returns the files written.
func CollectTraces(ctx context.Context, kubeClient kubernetes.Interface, proxy rest.Interface, opts CollectOptions, dir, namespace string, meta TraceMeta) ([]string, error) {
	pods, err := kubeClient.CoreV1().Pods(opts.OperatorNamespace).List(ctx, metav1.ListOptions{LabelSelector: opts.OperatorSelector})
	if err != nil {
		return nil, fmt.Errorf("can't list operator Pods: %w", err)
	}

	var written []string
	var errs []error
	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}

		body, found, err := FetchTrace(ctx, proxy, opts.OperatorNamespace, pod.Name, opts.Port, namespace, opts.Timeout)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !found || len(body) == 0 {
			continue
		}

		err = os.MkdirAll(dir, 0o755)
		if err != nil {
			return written, err
		}
		path := filepath.Join(dir, fmt.Sprintf("%s.%s.jsonl", namespace, pod.Name))
		err = os.WriteFile(path, body, 0o644)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		written = append(written, path)
	}

	if len(written) > 0 {
		meta.Namespace = namespace
		err = WriteTraceMeta(filepath.Join(dir, namespace+".meta.json"), meta)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return written, errors.Join(errs...)
}
