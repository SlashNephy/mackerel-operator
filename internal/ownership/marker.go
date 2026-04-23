package ownership

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const Heritage = "mackerel-operator"

var markerPattern = regexp.MustCompile(`<!--\s*heritage=mackerel-operator,resource=([^,\s]+),owner=([^,\s]+),hash=([^,\s]+)\s*-->`)

type Marker struct {
	Resource string
	Owner    string
	Hash     string
}

func BuildMarker(marker Marker) string {
	return fmt.Sprintf(
		"<!-- heritage=%s,resource=%s,owner=%s,hash=%s -->",
		Heritage,
		escapeMarkerValue(marker.Resource),
		escapeMarkerValue(marker.Owner),
		escapeMarkerValue(marker.Hash),
	)
}

func ParseMarker(memo string) (Marker, bool) {
	matches := markerPattern.FindStringSubmatch(memo)
	if matches == nil {
		return Marker{}, false
	}
	resource, err := unescapeMarkerValue(matches[1])
	if err != nil {
		return Marker{}, false
	}
	owner, err := unescapeMarkerValue(matches[2])
	if err != nil {
		return Marker{}, false
	}
	hash, err := unescapeMarkerValue(matches[3])
	if err != nil {
		return Marker{}, false
	}
	return Marker{
		Resource: resource,
		Owner:    owner,
		Hash:     hash,
	}, true
}

func ApplyMarker(memo string, marker Marker) string {
	base := RemoveMarker(memo)
	if base == "" {
		return BuildMarker(marker)
	}
	if strings.HasSuffix(base, "\n") {
		return base + BuildMarker(marker)
	}
	return base + "\n" + BuildMarker(marker)
}

func RemoveMarker(memo string) string {
	return markerPattern.ReplaceAllString(memo, "")
}

func escapeMarkerValue(value string) string {
	var builder strings.Builder
	for i := 0; i < len(value); i++ {
		b := value[i]
		if isMarkerValueSafe(b) {
			builder.WriteByte(b)
			continue
		}
		builder.WriteString(fmt.Sprintf("%%%02X", b))
	}
	return builder.String()
}

func unescapeMarkerValue(value string) (string, error) {
	var builder strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] != '%' {
			builder.WriteByte(value[i])
			continue
		}
		if i+2 >= len(value) {
			return "", fmt.Errorf("invalid percent escape")
		}
		decoded, err := strconv.ParseUint(value[i+1:i+3], 16, 8)
		if err != nil {
			return "", fmt.Errorf("invalid percent escape: %w", err)
		}
		builder.WriteByte(byte(decoded))
		i += 2
	}
	return builder.String(), nil
}

func isMarkerValueSafe(b byte) bool {
	switch {
	case 'a' <= b && b <= 'z':
		return true
	case 'A' <= b && b <= 'Z':
		return true
	case '0' <= b && b <= '9':
		return true
	case b == '-' || b == '_' || b == '.' || b == '/':
		return true
	default:
		return false
	}
}
