package main

import (
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
)

// addTopologySpreadConstraints performs the mutation(s) needed to add Topology Spread Constraints to your resource
func addTopologySpreadConstraints(target, TopologyConstraints []corev1.TopologySpreadConstraint, basePath string) (patch []patchOperation) {
	first := len(target) == 0
	if first {
		structuredLog(LogLevelDebug, "Topology", "No existing topology spread constraints found, will create new array")
	} else {
		structuredLog(LogLevelDebug, "Topology", "Found %d existing topology spread constraints", len(target))
	}

	var value interface{}
	for _, tsc := range TopologyConstraints {
		value = tsc
		path := basePath
		var skip bool
		var op string
		if first {
			first = false
			op = "add"
			value = []corev1.TopologySpreadConstraint{tsc}
			structuredLog(LogLevelDebug, "Topology", "Adding first topology spread constraint with key: %s", tsc.TopologyKey)
		} else {
			optExists := false
			for idx, targetOpt := range target {
				keyEqual := cmp.Equal(targetOpt.TopologyKey, tsc.TopologyKey)
				if keyEqual {
					optExists = true
					skewEqual := cmp.Equal(targetOpt.MaxSkew, tsc.MaxSkew)
					nodeAffinityEqual := cmp.Equal(targetOpt.NodeAffinityPolicy, tsc.NodeAffinityPolicy)
					nodeTaintEqual := cmp.Equal(targetOpt.NodeTaintsPolicy, tsc.NodeTaintsPolicy)
					unsatisfiableEqual := cmp.Equal(targetOpt.WhenUnsatisfiable, tsc.WhenUnsatisfiable)
					labelSelectorEqual := cmp.Equal(targetOpt.LabelSelector, tsc.LabelSelector)
					matchLabelKeysEqual := cmp.Equal(targetOpt.MatchLabelKeys, tsc.MatchLabelKeys)

					skip, op, path = checkReplaceOrSkip(idx, path, skewEqual, nodeAffinityEqual, nodeTaintEqual, unsatisfiableEqual, labelSelectorEqual, matchLabelKeysEqual)
					if !skip {
						structuredLog(LogLevelInfo, "Topology", "Updating existing topology spread constraint at index %d with key: %s", idx, tsc.TopologyKey)
					} else {
						structuredLog(LogLevelDebug, "Topology", "Skipping topology spread constraint update at index %d with key: %s (no changes needed)", idx, tsc.TopologyKey)
					}
				}
			}
			if !optExists {
				op = "add"
				path = path + "/-"
				structuredLog(LogLevelInfo, "Topology", "Adding new topology spread constraint with key: %s", tsc.TopologyKey)
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
