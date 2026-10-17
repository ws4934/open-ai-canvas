package programupdate

import (
	"strconv"
	"strings"
)

// Releases use a numeric fourth component as well as the usual three-part form.
func parseVersion(raw string) ([]uint64, bool) {
	parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(raw), "v"), ".")
	if len(parts) != 3 && len(parts) != 4 {
		return nil, false
	}
	out := make([]uint64, 4)
	for i, part := range parts {
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return nil, false
		}
		for _, c := range part {
			if c < '0' || c > '9' {
				return nil, false
			}
		}
		value, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return nil, false
		}
		out[i] = value
	}
	return out, true
}

func IsReleaseVersion(value string) bool { _, ok := parseVersion(value); return ok }

func CompareVersions(left, right string) int {
	a, aok := parseVersion(left)
	b, bok := parseVersion(right)
	if !aok && bok {
		return -1
	}
	if aok && !bok {
		return 1
	}
	if !aok {
		return strings.Compare(left, right)
	}
	for i := range a {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

func releaseTag(value string) string { return "v" + strings.TrimPrefix(strings.TrimSpace(value), "v") }
