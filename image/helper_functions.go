package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// LogLevel 定义日志级别
type LogLevel string

const (
	LogLevelInfo    LogLevel = "INFO"
	LogLevelWarning LogLevel = "WARNING"
	LogLevelError   LogLevel = "ERROR"
	LogLevelDebug   LogLevel = "DEBUG"
)

// structuredLog 输出结构化日志
func structuredLog(level LogLevel, component string, format string, args ...interface{}) {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	message := fmt.Sprintf(format, args...)
	logMessage := fmt.Sprintf("[%s] [%s] [%s] %s", timestamp, level, component, message)

	switch level {
	case LogLevelError:
		glog.ErrorDepth(1, logMessage)
	case LogLevelWarning:
		glog.WarningDepth(1, logMessage)
	case LogLevelDebug:
		glog.V(2).InfoDepth(1, logMessage)
	default:
		glog.InfoDepth(1, logMessage)
	}
}

func loadConfig(configFile string) (*Config, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}
	structuredLog(LogLevelInfo, "Config", "新配置文件校验和: sha256sum %x", sha256.Sum256(data))

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	structuredLog(LogLevelDebug, "Config", "配置数据: %+v", &cfg)

	return &cfg, nil
}

// mutationRequired checks whether the target resource needs to be mutated.
// Mutation is enabled by default unless explicitly disabled.
func mutationRequired(ignoredList []string, metadata *metav1.ObjectMeta, config *Config) bool {
	// skip excluded kubernetes system namespaces
	for _, namespace := range ignoredList {
		if metadata.Namespace == namespace {
			structuredLog(LogLevelInfo, "Mutation", "跳过命名空间 %v 中的 %v 的变更", metadata.Namespace, metadata.Name)
			return false
		}
	}

	annotations := metadata.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	// 检查是否已经注入
	if strings.ToLower(annotations[admissionWebhookAnnotationStatusKey]) == "injected" {
		structuredLog(LogLevelInfo, "Mutation", "跳过 %v/%v 的变更: 已经注入", metadata.Namespace, metadata.Name)
		return false
	}

	// 检查是否明确禁用注入
	if val := annotations[admissionWebhookAnnotationInjectKey]; strings.ToLower(val) == "no" ||
		strings.ToLower(val) == "false" || strings.ToLower(val) == "off" {
		structuredLog(LogLevelInfo, "Mutation", "跳过 %v/%v 的变更: 明确禁用注入", metadata.Namespace, metadata.Name)
		return false
	}

	// 如果配置了Pod选择器，检查Pod是否匹配
	if config != nil && config.PodSelector != nil {
		selector, err := metav1.LabelSelectorAsSelector(config.PodSelector)
		if err != nil {
			structuredLog(LogLevelError, "Mutation", "无效的 pod 选择器: %v", err)
			return false
		}

		// 检查Pod的标签是否匹配选择器
		if !selector.Matches(labels.Set(metadata.Labels)) {
			structuredLog(LogLevelInfo, "Mutation", "Pod %s/%s 不匹配标签选择器", metadata.Namespace, metadata.Name)
			return false
		}
	}

	structuredLog(LogLevelInfo, "Mutation", "需要对 %v/%v 进行变更", metadata.Namespace, metadata.Name)
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
