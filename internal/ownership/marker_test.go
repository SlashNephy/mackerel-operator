package ownership

import "testing"

func TestBuildMarker(t *testing.T) {
	got := BuildMarker(Marker{
		Resource: "externalmonitor/default/api-health",
		Owner:    "prod",
		Hash:     "deadbee",
	})
	want := "<!-- heritage=mackerel-operator,resource=externalmonitor/default/api-health,owner=prod,hash=deadbee -->"
	if got != want {
		t.Fatalf("BuildMarker() = %q, want %q", got, want)
	}
}

func TestParseMarker(t *testing.T) {
	memo := "human memo\n<!-- heritage=mackerel-operator,resource=externalmonitor/default/api-health,owner=prod,hash=deadbee -->"
	got, ok := ParseMarker(memo)
	if !ok {
		t.Fatal("ParseMarker ok = false, want true")
	}
	if got.Resource != "externalmonitor/default/api-health" || got.Owner != "prod" || got.Hash != "deadbee" {
		t.Fatalf("ParseMarker() = %#v", got)
	}
}

func TestApplyMarkerPreservesHumanMemo(t *testing.T) {
	got := ApplyMarker("human memo", Marker{
		Resource: "externalmonitor/default/api-health",
		Owner:    "prod",
		Hash:     "deadbee",
	})
	want := "human memo\n<!-- heritage=mackerel-operator,resource=externalmonitor/default/api-health,owner=prod,hash=deadbee -->"
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
	want := "  human memo  \n<!-- heritage=mackerel-operator,resource=externalmonitor/default/api-health,owner=prod,hash=deadbee -->"
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
	want := "human memo\n\n<!-- heritage=mackerel-operator,resource=externalmonitor/default/api-health,owner=prod,hash=deadbee -->"
	if got != want {
		t.Fatalf("ApplyMarker() = %q, want %q", got, want)
	}
}

func TestRemoveMarker(t *testing.T) {
	memo := "human memo\n<!-- heritage=mackerel-operator,resource=externalmonitor/default/api-health,owner=prod,hash=deadbee -->"
	got := RemoveMarker(memo)
	if got != "human memo\n" {
		t.Fatalf("RemoveMarker() = %q, want human memo\\n", got)
	}
}

func TestRemoveMarkerPreservesHumanMemoWhitespace(t *testing.T) {
	memo := "  human memo  \n<!-- heritage=mackerel-operator,resource=externalmonitor/default/api-health,owner=prod,hash=deadbee -->"
	got := RemoveMarker(memo)
	want := "  human memo  \n"
	if got != want {
		t.Fatalf("RemoveMarker() = %q, want %q", got, want)
	}
}
