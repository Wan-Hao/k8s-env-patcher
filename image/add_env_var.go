package main

import (
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
)

// addEnv performs the mutation(s) needed to add the extra environment variables to the target
// resource
func addEnv(target, envVars []corev1.EnvVar, basePath string) (patch []patchOperation) {
	first := len(target) == 0
	if first {
		structuredLog(LogLevelDebug, "EnvVars", "No existing environment variables found, will create new array")
	} else {
		structuredLog(LogLevelDebug, "EnvVars", "Found %d existing environment variables", len(target))
	}

	var value interface{}
	for _, envVar := range envVars {
		value = envVar
		path := basePath
		var skip bool
		var op string
		if first {
			first = false
			op = "add"
			value = []corev1.EnvVar{envVar}
			structuredLog(LogLevelDebug, "EnvVars", "Adding first environment variable: %s", envVar.Name)
		} else {
			optExists := false
			for idx, targetOpt := range target {
				nameEqual := cmp.Equal(targetOpt.Name, envVar.Name)
				if nameEqual {
					optExists = true
					valueEqual := cmp.Equal(targetOpt.Value, envVar.Value)
					valueFromEqual := cmp.Equal(targetOpt.ValueFrom, envVar.ValueFrom)

					skip, op, path = checkReplaceOrSkip(idx, path, valueEqual, valueFromEqual)
					if !skip {
						structuredLog(LogLevelInfo, "EnvVars", "Updating existing environment variable at index %d: %s", idx, envVar.Name)
					} else {
						structuredLog(LogLevelDebug, "EnvVars", "Skipping environment variable update at index %d: %s (no changes needed)", idx, envVar.Name)
					}
				}
			}
			if !optExists {
				op = "add"
				path = path + "/-"
				structuredLog(LogLevelInfo, "EnvVars", "Adding new environment variable: %s", envVar.Name)
			}
		}
		if !skip {
			patch = append(patch, patchOperation{
				Op:    op,
				Path:  path,
				Value: value,
			})
		} else {
			patch = []patchOperation{}
		}
	}
	return patch
}
