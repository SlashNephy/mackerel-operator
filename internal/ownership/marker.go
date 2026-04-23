package ownership

import (
	"fmt"
	"regexp"
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
	return fmt.Sprintf("<!-- heritage=%s,resource=%s,owner=%s,hash=%s -->", Heritage, marker.Resource, marker.Owner, marker.Hash)
}

func ParseMarker(memo string) (Marker, bool) {
	matches := markerPattern.FindStringSubmatch(memo)
	if matches == nil {
		return Marker{}, false
	}
	return Marker{
		Resource: matches[1],
		Owner:    matches[2],
		Hash:     matches[3],
	}, true
}

func ApplyMarker(memo string, marker Marker) string {
	base := RemoveMarker(memo)
	if strings.TrimSpace(base) == "" {
		return BuildMarker(marker)
	}
	return strings.TrimRight(base, "\n") + "\n" + BuildMarker(marker)
}

func RemoveMarker(memo string) string {
	without := markerPattern.ReplaceAllString(memo, "")
	return strings.TrimRight(strings.TrimSpace(without), "\n")
}
