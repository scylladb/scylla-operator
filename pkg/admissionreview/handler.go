package admissionreview

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	apimachineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
)

var (
	scheme = runtime.NewScheme()
	codecs serializer.CodecFactory
)

func init() {
	apimachineryutilruntime.Must(admissionv1.AddToScheme(scheme))

	codecs = serializer.NewCodecFactory(scheme)
}

// HandleFunc is a function type that processes an AdmissionReview request and returns warnings and a validation error.
type HandleFunc func(*admissionv1.AdmissionReview) ([]string, error)

type handler struct {
	f HandleFunc
}

var _ http.Handler = &handler{}

func NewHandler(f HandleFunc) *handler {
	return &handler{
		f: f,
	}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var body []byte
	if req.Body != nil {
		data, err := ioutil.ReadAll(req.Body)
		if err == nil {
			body = data
		}
	}

	contentType := req.Header.Get("Content-Type")
	if contentType != "application/json" {
		msg := fmt.Sprintf("unsupported contentType %q", contentType)
		klog.Error(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	klog.V(4).Info(fmt.Sprintf("handling request: %s", body))

	deserializer := codecs.UniversalDeserializer()
	obj, gvk, err := deserializer.Decode(body, nil, nil)
	if err != nil {
		msg := fmt.Sprintf("Request could not be decoded: %v", err)
		klog.Error(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	var responseObj runtime.Object
	switch *gvk {
	case admissionv1.SchemeGroupVersion.WithKind("AdmissionReview"):
		requestedAdmissionReview, ok := obj.(*admissionv1.AdmissionReview)
		if !ok {
			msg := fmt.Sprintf("Expected v1.AdmissionReview but got: %T", obj)
			klog.Error(msg)
			http.Error(w, msg, http.StatusBadRequest)
			return
		}

		funcWarnings, funcErr := h.f(requestedAdmissionReview)
		if funcErr != nil {
			klog.V(2).InfoS("Review failed", "Error", err)
		}

		responseAdmissionReview := &admissionv1.AdmissionReview{}
		responseAdmissionReview.SetGroupVersionKind(*gvk)
		responseAdmissionReview.Response = &admissionv1.AdmissionResponse{
			UID:     requestedAdmissionReview.Request.UID,
			Allowed: funcErr == nil,
			Result: func() *metav1.Status {
				s, ok := funcErr.(apierrors.APIStatus)
				if ok {
					sts := s.Status()
					return &sts
				}

				return &metav1.Status{
					Message: func() string {
						if funcErr == nil {
							return ""
						}
						return funcErr.Error()
					}(),
				}
			}(),
			Warnings: funcWarnings,
		}
		responseObj = responseAdmissionReview

	default:
		msg := fmt.Sprintf("Unsupported GVK: %v", gvk)
		klog.Error(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	respBytes, err := json.Marshal(responseObj)
	if err != nil {
		klog.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	klog.V(4).Infof("sending response: %v", responseObj)
	_, err = w.Write(respBytes)
	if err != nil {
		klog.Error(err)
	}
}
