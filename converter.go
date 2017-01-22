package configuration

import (
	"sort"
	"strings"
)

// appendToNamespace appends the given label to the given namespace. This
// operates against a contract with labelsToNamespace. Both methods use the dash
// as separators.
func appendToNamespace(namespace, label string) string {
	return namespace + "-" + label
}

func mappingsToKeys(m map[string][]interface{}) []string {
	var keys []string

	for k, _ := range m {
		keys = append(keys, k)
	}

	return keys
}

// labelsToNamespace creates a reproducible namespace using the given labels.
// This operates against a contract with appendToNamespace. Both methods use the
// dash as separators.
func labelsToNamespace(labels ...string) string {
	sort.Strings(labels)

	namespace := strings.Join(labels, "-")

	return namespace
}
