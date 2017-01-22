package configuration

import (
	"fmt"
)

func namespaceKey(namespace string) string {
	return fmt.Sprintf("service:configuration:namespace:%s", namespace)
}

func pieceListKey(namespace string) string {
	return fmt.Sprintf("%s:piece:list", namespaceKey(namespace))
}

// pieceUsedKey is to hold the piece ID of the piece being used recently for the
// given namespace.
func pieceUsedKey(namespace string) string {
	return fmt.Sprintf("%s:piece:used", namespaceKey(namespace))
}

func rulerListKey(namespace string) string {
	return fmt.Sprintf("%s:ruler:list", namespaceKey(namespace))
}

// rulerUsedKey is to hold the ruler ID of the ruler being used recently for the
// given namespace.
func rulerUsedKey(namespace string) string {
	return fmt.Sprintf("%s:ruler:used", namespaceKey(namespace))
}
