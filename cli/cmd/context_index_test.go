package cmd

import (
	"reflect"
	"testing"
)

func TestContextIndexDirsDerivedFromRegistry(t *testing.T) {
	var want []string
	for _, typ := range contextTypeList {
		if typ.tier != "" || typ.statusRow {
			want = append(want, typ.dir)
		}
	}

	got := ctxIndexDirsForTypes(contextTypeList)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ctxIndexDirsForTypes(contextTypeList) = %v, want registry order %v", got, want)
	}
	if !reflect.DeepEqual(ctxIndexDirs, got) {
		t.Fatalf("ctxIndexDirs = %v, want derived projection %v", ctxIndexDirs, got)
	}

	contains := func(dir string) bool {
		for _, gotDir := range got {
			if gotDir == dir {
				return true
			}
		}
		return false
	}
	if !contains(sdtBriefingDir) {
		t.Error("briefing has a tier/status row but is missing from the corpus walk")
	}
	if contains(sdtWikiDir) {
		t.Error("wiki must remain on its dedicated linter, outside the corpus walk")
	}
	if contains(sdtTmpDir) {
		t.Error("tmp scratch must remain outside the corpus walk")
	}
}

func TestEveryContextTypeHasOneExplicitIndexLintRoute(t *testing.T) {
	for _, typ := range contextTypeList {
		got := ctxIndexRouteForType(typ)
		switch got {
		case ctxIndexRouteCorpus:
			if typ.tier == "" && !typ.statusRow {
				t.Errorf("%s routes to corpus without a tier or status row", typ.kind)
			}
		case ctxIndexRouteDedicated:
			if typ.kind != ctxTypeWiki {
				t.Errorf("%s unexpectedly routes to the dedicated linter", typ.kind)
			}
		case ctxIndexRouteExcluded:
			if typ.kind != ctxTypeTmp {
				t.Errorf("%s unexpectedly excluded from the corpus walk", typ.kind)
			}
		default:
			t.Errorf("registry kind %s has no lint/index route", typ.kind)
		}
	}
	if got := ctxIndexRouteForType(ctxDocType{kind: "future-kind"}); got != ctxIndexRouteUnknown {
		t.Errorf("unregistered-shape kind route = %q, want unclassified", got)
	}
}
