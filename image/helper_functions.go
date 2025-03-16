package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func loadConfig(configFile string) (*Config, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}
	glog.Infof("New configuration: sha256sum %x", sha256.Sum256(data))

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	glog.Infof("Configuration data: %+v", &cfg)

	return &cfg, nil
}

// mutationRequired checks whether the target resource needs to be mutated.
// Mutation is enabled by default unless explicitly disabled.
func mutationRequired(ignoredList []string, metadata *metav1.ObjectMeta, config *Config) bool {
	// skip excluded kubernetes system namespaces
	for _, namespace := range ignoredList {
		if metadata.Namespace == namespace {
			glog.Infof("Skip mutation for %v in namespace: %v", metadata.Name, metadata.Namespace)
			return false
		}
	}

	annotations := metadata.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	// 检查是否已经注入
	if strings.ToLower(annotations[admissionWebhookAnnotationStatusKey]) == "injected" {
		glog.Infof("Skip mutation for %v/%v: already injected", metadata.Namespace, metadata.Name)
		return false
	}

	// 检查是否明确禁用注入
	if val := annotations[admissionWebhookAnnotationInjectKey]; strings.ToLower(val) == "no" ||
		strings.ToLower(val) == "false" || strings.ToLower(val) == "off" {
		glog.Infof("Skip mutation for %v/%v: injection explicitly disabled", metadata.Namespace, metadata.Name)
		return false
	}

	// 如果配置了Pod选择器，检查Pod是否匹配
	if config != nil && config.PodSelector != nil {
		selector, err := metav1.LabelSelectorAsSelector(config.PodSelector)
		if err != nil {
			glog.Errorf("Invalid pod selector: %v", err)
			return false
		}

		// 检查Pod的标签是否匹配选择器
		if !selector.Matches(labels.Set(metadata.Labels)) {
			glog.Infof("Pod %s/%s does not match the label selector", metadata.Namespace, metadata.Name)
			return false
		}
	}

	glog.Infof("Mutation required for %v/%v", metadata.Namespace, metadata.Name)
	return true
}

func updateAnnotation(target map[string]string, annotations map[string]string) (patch []patchOperation) {
	for k, v := range annotations {
		if target == nil {
			target = map[string]string{}
			patch = append(patch, patchOperation{
				Op:   "add",
				Path: "/metadata/annotations",
				Value: map[string]string{
					k: v,
				},
			})
		} else if target[k] == "" {
			target = map[string]string{}
			patch = append(patch, patchOperation{
				Op:    "add",
				Path:  "/metadata/annotations/" + k,
				Value: v,
			})
		} else {
			patch = append(patch, patchOperation{
				Op:    "replace",
				Path:  "/metadata/annotations/" + k,
				Value: v,
			})
		}
	}
	return patch
}

// function to test conditions pased in and determine if we need to replace existing config or skip it when it matches
func checkReplaceOrSkip(idx int, inPath string, conditions ...bool) (skip bool, op, path string) {

	for _, condition := range conditions {
		if !condition {
			op = "replace"
			path = fmt.Sprintf("%s/%d", inPath, idx)
			skip = false
			return
		}
	}

	// If we reach this point, all conditions are true
	skip = true // We skip only if all conditions are true
	return

}
