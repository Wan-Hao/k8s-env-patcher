package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	v1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()
)

var ignoredNamespaces = []string{
	metav1.NamespaceSystem,
	metav1.NamespacePublic,
}

const (
	admissionWebhookAnnotationInjectKey = "env-injector-webhook-inject"
	admissionWebhookAnnotationStatusKey = "env-injector-webhook-status"
)

type WebhookServer struct {
	envConfig *Config
	server    *http.Server
}

// Webhook Server parameters
type WhSvrParameters struct {
	port       int    // webhook server port
	certFile   string // path to the x509 certificate for https
	keyFile    string // path to the x509 private key matching `CertFile`
	envCfgFile string // path to env injector configuration file
}

type Config struct {
	Env                        []corev1.EnvVar                   `yaml:"env"`
	DnsOptions                 []corev1.PodDNSConfigOption       `yaml:"dnsOptions,omitempty"`
	RequiredNodeAffinityTerms  []corev1.NodeSelectorTerm         `yaml:"requiredNodeAffinityTerms,omitempty"`
	PreferredNodeAffinityTerms []corev1.PreferredSchedulingTerm  `yaml:"preferredNodeAffinityTerms,omitempty"`
	Tolerations                []corev1.Toleration               `yaml:"tolerations,omitempty"`
	TopologyConstraints        []corev1.TopologySpreadConstraint `yaml:"topologyConstraints,omitempty"`
	RemovePodAntiAffinity      bool                              `yaml:"removePodAntiAffinity,omitempty"`
	PodSelector                *metav1.LabelSelector             `yaml:"podSelector,omitempty"`
}

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func init() {
	_ = corev1.AddToScheme(runtimeScheme)
	_ = admissionregistrationv1.AddToScheme(runtimeScheme)
}

// main mutation process
func (whsvr *WebhookServer) mutate(ar *v1.AdmissionReview) *v1.AdmissionResponse {
	req := ar.Request
	var pod corev1.Pod
	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		structuredLog(LogLevelError, "Webhook", "无法解析原始对象: %v", err)
		return &v1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	structuredLog(LogLevelInfo, "Webhook", "收到准入审查请求 Kind=%v, Namespace=%v Name=%v (%v) UID=%v Operation=%v UserInfo=%v",
		req.Kind, req.Namespace, req.Name, pod.Name, req.UID, req.Operation, req.UserInfo)

	// determine whether to perform mutation
	if !mutationRequired(ignoredNamespaces, &pod.ObjectMeta, whsvr.envConfig) {
		structuredLog(LogLevelInfo, "Webhook", "根据策略检查跳过对 %s/%s 的变更", pod.Namespace, pod.Name)
		return &v1.AdmissionResponse{
			Allowed: true,
		}
	}

	annotations := map[string]string{admissionWebhookAnnotationStatusKey: "injected"}
	patchBytes, err := createPatch(&pod, whsvr.envConfig, annotations)
	if err != nil {
		return &v1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	structuredLog(LogLevelDebug, "Webhook", "准入响应补丁内容: %s", string(patchBytes))
	return &v1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *v1.PatchType {
			pt := v1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

// serve manages requests to the webhook server
func (whsvr *WebhookServer) serve(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := io.ReadAll(r.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		structuredLog(LogLevelError, "Webhook", "请求体为空")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		structuredLog(LogLevelError, "Webhook", "Content-Type=%s, 期望 application/json", contentType)
		http.Error(w, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	var admissionResponse *v1.AdmissionResponse
	ar := v1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		structuredLog(LogLevelError, "Webhook", "无法解码请求体: %v", err)
		admissionResponse = &v1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {
		admissionResponse = whsvr.mutate(&ar)
	}

	admissionReview := v1.AdmissionReview{}
	if admissionResponse != nil {
		admissionReview.Response = admissionResponse
		if ar.Request != nil {
			admissionReview.Response.UID = ar.Request.UID
		}
	}

	resp, err := json.Marshal(admissionReview)
	if err != nil {
		structuredLog(LogLevelError, "Webhook", "无法编码响应: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}
	structuredLog(LogLevelInfo, "Webhook", "准备写入响应...")
	if _, err := w.Write(resp); err != nil {
		structuredLog(LogLevelError, "Webhook", "无法写入响应: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}
