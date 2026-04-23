package ownership

import "testing"

const deadbeeMarker = "<!-- heritage=mackerel-operator,resource=externalmonitor/default/api-health,owner=prod,hash=deadbee -->"

func TestBuildMarker(t *testing.T) {
	got := BuildMarker(Marker{
		Resource: "externalmonitor/default/api-health",
		Owner:    "prod",
		Hash:     "deadbee",
	})
	want := deadbeeMarker
	if got != want {
		t.Fatalf("BuildMarker() = %q, want %q", got, want)
	}
}

func TestBuildMarkerEscapesUnsafeValues(t *testing.T) {
	got := BuildMarker(Marker{
		Resource: "externalmonitor/default/api-health",
		Owner:    "team a,blue",
		Hash:     "dead bee%",
	})
	want := "<!-- heritage=mackerel-operator,resource=externalmonitor/default/api-health,owner=team%20a%2Cblue,hash=dead%20bee%25 -->"
	if got != want {
		t.Fatalf("BuildMarker() = %q, want %q", got, want)
	}
}

func TestParseMarker(t *testing.T) {
	memo := "human memo\n" + deadbeeMarker
	got, ok := ParseMarker(memo)
	if !ok {
		t.Fatal("ParseMarker ok = false, want true")
	}
	if got.Resource != "externalmonitor/default/api-health" || got.Owner != "prod" || got.Hash != "deadbee" {
		t.Fatalf("ParseMarker() = %#v", got)
	}
}

func TestParseMarkerRoundTripsEscapedValues(t *testing.T) {
	marker := Marker{
		Resource: "externalmonitor/default/api-health",
		Owner:    "team a,blue",
		Hash:     "dead bee%",
	}

	got, ok := ParseMarker(BuildMarker(marker))
	if !ok {
		t.Fatal("ParseMarker ok = false, want true")
	}
	if got != marker {
		t.Fatalf("ParseMarker() = %#v, want %#v", got, marker)
	}
}

func TestParseMarkerRejectsInvalidEscapes(t *testing.T) {
	_, ok := ParseMarker("<!-- heritage=mackerel-operator,resource=externalmonitor/default/api-health,owner=%zz,hash=deadbee -->")
	if ok {
		t.Fatal("ParseMarker ok = true, want false")
	}
}

func TestApplyMarkerPreservesHumanMemo(t *testing.T) {
	got := ApplyMarker("human memo", Marker{
		Resource: "externalmonitor/default/api-health",
		Owner:    "prod",
		Hash:     "deadbee",
	})
	want := "human memo\n" + deadbeeMarker
	if got != want {
		t.Fatalf("ApplyMarker() = %q, want %q", got, want)
	}
}

func TestApplyMarkerReplacesExistingMarker(t *testing.T) {
	memo := "human memo\n<!-- heritage=mackerel-operator,resource=externalmonitor/default/api-health,owner=prod,hash=oldhash -->"
	got := ApplyMarker(memo, Marker{
		Resource: "externalmonitor/default/api-health",
		Owner:    "prod",
		Hash:     "newhash",
	})
	if got != "human memo\n<!-- heritage=mackerel-operator,resource=externalmonitor/default/api-health,owner=prod,hash=newhash -->" {
		t.Fatalf("ApplyMarker() = %q", got)
	}
}

func TestApplyMarkerPreservesBlankLinesBeforeExistingMarker(t *testing.T) {
	memo := "human memo\n\n<!-- heritage=mackerel-operator,resource=externalmonitor/default/api-health,owner=prod,hash=oldhash -->"
	got := ApplyMarker(memo, Marker{
		Resource: "externalmonitor/default/api-health",
		Owner:    "prod",
		Hash:     "newhash",
	})
	want := "human memo\n\n<!-- heritage=mackerel-operator,resource=externalmonitor/default/api-health,owner=prod,hash=newhash -->"
	if got != want {
		t.Fatalf("ApplyMarker() = %q, want %q", got, want)
	}
}

func TestApplyMarkerPreservesHumanMemoWhitespace(t *testing.T) {
	memo := "  human memo  "
	got := ApplyMarker(memo, Marker{
		Resource: "externalmonitor/default/api-health",
		Owner:    "prod",
		Hash:     "deadbee",
	})
	want := "  human memo  \n" + deadbeeMarker
	if got != want {
		t.Fatalf("ApplyMarker() = %q, want %q", got, want)
	}
}

func TestApplyMarkerPreservesTrailingNewlines(t *testing.T) {
	memo := "human memo\n\n"
	got := ApplyMarker(memo, Marker{
		Resource: "externalmonitor/default/api-health",
		Owner:    "prod",
		Hash:     "deadbee",
	})
	want := "human memo\n\n" + deadbeeMarker
	if got != want {
		t.Fatalf("ApplyMarker() = %q, want %q", got, want)
	}
}

func TestRemoveMarker(t *testing.T) {
	memo := "human memo\n" + deadbeeMarker
	got := RemoveMarker(memo)
	if got != "human memo\n" {
		t.Fatalf("RemoveMarker() = %q, want human memo\\n", got)
	}
}

func TestRemoveMarkerPreservesHumanMemoWhitespace(t *testing.T) {
	memo := "  human memo  \n" + deadbeeMarker
	got := RemoveMarker(memo)
	want := "  human memo  \n"
	if got != want {
		t.Fatalf("RemoveMarker() = %q, want %q", got, want)
	}
}
