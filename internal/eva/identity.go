package eva

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/prometheus/prometheus/model/labels"
)

// IdentityKey builds a stable hash from configured identity labels.
func IdentityKey(identityLabels []string, ls labels.Labels) (string, error) {
	pairs := make([]string, 0, len(identityLabels))
	for _, name := range identityLabels {
		if v := ls.Get(name); v != "" {
			pairs = append(pairs, name+"="+v)
		}
	}
	if len(pairs) == 0 {
		return "", fmt.Errorf("none of identity labels %v present on alert group", identityLabels)
	}
	sort.Strings(pairs)
	sum := sha256.Sum256([]byte(strings.Join(pairs, "\n")))
	return hex.EncodeToString(sum[:]), nil
}

// LabelsMap converts prometheus labels to a plain map for templates / routes.
func LabelsMap(ls labels.Labels) map[string]string {
	out := make(map[string]string, ls.Len())
	ls.Range(func(l labels.Label) {
		out[l.Name] = l.Value
	})
	return out
}

// ResolveTarget returns route match or defaultTarget.
func ResolveTarget(defaultTarget string, routes []Route, labelMap map[string]string) string {
	for _, r := range routes {
		if labelMap[r.MatchLabel] == r.MatchValue {
			return r.Target
		}
	}
	return defaultTarget
}

// Route is a simplified routing rule used by the service layer.
type Route struct {
	MatchLabel string
	MatchValue string
	Target     string
}
