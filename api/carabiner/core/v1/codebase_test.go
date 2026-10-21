// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"slices"
	"testing"
)

const (
	langGo         = "golang"
	dirBackend     = "backend"
	dirServicesAPI = "services/api"
	idGoBackend    = "golang:backend"
	slugWidgets    = "acme/widgets"
	sysGitHub      = "github"
)

func sortStrings(s []string) { slices.Sort(s) }

func equalStrings(a, b []string) bool { return slices.Equal(a, b) }

func TestCanonicalCodebasePath(t *testing.T) {
	cases := map[string]string{
		".":               CodebaseRootPath,
		"./":              CodebaseRootPath,
		"backend":         dirBackend,
		"./backend":       dirBackend,
		"backend/":        dirBackend,
		"./services/api":  dirServicesAPI,
		"services//api":   dirServicesAPI,
		"services/./api":  dirServicesAPI,
		"a/../b":          "b",
		"services/api/..": "services",
	}
	for in, want := range cases {
		got, err := CanonicalCodebasePath(in)
		if err != nil {
			t.Errorf("%q: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("CanonicalCodebasePath(%q) = %q, want %q", in, got, want)
		}
	}
	for _, in := range []string{"", "/backend", "..", "../x", "a/../../x", `services\api`} {
		if _, err := CanonicalCodebasePath(in); !errors.Is(err, ErrInvalidCodebaseID) {
			t.Errorf("%q must be refused, got %v", in, err)
		}
	}
}

func TestValidateCodebaseLanguage(t *testing.T) {
	for _, l := range []string{langGo, "python", "npm", "c", "dotnet-8", "node_js", "v1.2"} {
		if err := ValidateCodebaseLanguage(l); err != nil {
			t.Errorf("%q must be valid: %v", l, err)
		}
		// The language is a label value as is.
		if err := ValidateLabelValue(l); err != nil {
			t.Errorf("%q must be a valid label value: %v", l, err)
		}
	}
	for _, l := range []string{"", "Go", "c++", "-go", "go-", "go lang", "go/lang"} {
		if err := ValidateCodebaseLanguage(l); !errors.Is(err, ErrInvalidCodebaseID) {
			t.Errorf("%q must be invalid, got %v", l, err)
		}
	}
}

func TestCodebaseSourceID(t *testing.T) {
	// The forms unpack writes and the canonical one give the same id.
	for _, in := range []string{"golang:./backend", idGoBackend, "golang:backend/", "golang:./backend/."} {
		language, p, err := ParseCodebaseSourceID(in)
		if err != nil {
			t.Errorf("ParseCodebaseSourceID(%q): %v", in, err)
			continue
		}
		if language != langGo || p != dirBackend {
			t.Errorf("ParseCodebaseSourceID(%q) = %q, %q, want golang, backend", in, language, p)
		}
		id, err := CodebaseSourceID(language, p)
		if err != nil || id != langGo+":"+dirBackend {
			t.Errorf("CodebaseSourceID(%q, %q) = %q, %v, want golang:backend", language, p, id, err)
		}
	}
	if id, err := CodebaseSourceID("npm", "./"); err != nil || id != "npm:." {
		t.Errorf("root codebase id = %q, %v, want npm:.", id, err)
	}
	for _, in := range []string{"", "golang", ":backend", "Go:backend", "golang:", "golang:/backend", "golang:../x"} {
		if _, _, err := ParseCodebaseSourceID(in); !errors.Is(err, ErrInvalidCodebaseID) {
			t.Errorf("%q must be refused, got %v", in, err)
		}
	}
	if CodebaseLanguageLabelKey != "carabiner.dev/language" || !IsReservedLabelKey(CodebaseLanguageLabelKey) {
		t.Errorf("language label key = %q, want a reserved key", CodebaseLanguageLabelKey)
	}
}

func TestCodebaseURI(t *testing.T) {
	cases := []struct {
		in   string
		want CodebaseURI
	}{
		{"codebase://github/acme/widgets#golang:backend", CodebaseURI{System: sysGitHub, Slug: slugWidgets, SourceID: idGoBackend}},
		{"codebase://github/acme/widgets@3f2a9c#golang:.", CodebaseURI{System: sysGitHub, Slug: slugWidgets, Revision: "3f2a9c", SourceID: "golang:."}},
		// A GitLab subgroup: the slug holds several segments, the fragment and
		// the revision still split unambiguously.
		{"codebase://gitlab/platform/tools/widgets@v1.2.3#npm:web/app", CodebaseURI{System: "gitlab", Slug: "platform/tools/widgets", Revision: "v1.2.3", SourceID: "npm:web/app"}},
		// unpack's older spelling canonicalizes.
		{"codebase://github/acme/widgets#golang:./backend/", CodebaseURI{System: sysGitHub, Slug: slugWidgets, SourceID: idGoBackend}},
	}
	for _, c := range cases {
		got, err := ParseCodebaseURI(c.in)
		if err != nil {
			t.Errorf("ParseCodebaseURI(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseCodebaseURI(%q) = %+v, want %+v", c.in, got, c.want)
		}
		if back := got.String(); back != got.String() || ParseMust(t, back) != got {
			t.Errorf("%q does not round-trip: %q", c.in, back)
		}
	}
	if got := (CodebaseURI{System: sysGitHub, Slug: slugWidgets, Revision: "abc", SourceID: "golang:."}).WithoutRevision().String(); got != "codebase://github/acme/widgets#golang:." {
		t.Errorf("WithoutRevision = %q", got)
	}
	if u, err := NewCodebaseURI(sysGitHub, slugWidgets, "abc", "golang:./backend"); err != nil || u.String() != "codebase://github/acme/widgets@abc#golang:backend" {
		t.Errorf("NewCodebaseURI = %q, %v", u, err)
	}
	for _, bad := range []string{
		"", "repository://github/acme/widgets#golang:.", "codebase://github/acme/widgets", "codebase://github#golang:.",
		"codebase://github/acme/widgets@#golang:.", "codebase://github/acme/widgets#golang:/abs", "codebase://Git Hub/acme/widgets#golang:.",
		"codebase://github/acme//widgets#golang:.",
	} {
		if _, err := ParseCodebaseURI(bad); !errors.Is(err, ErrInvalidCodebaseURI) {
			t.Errorf("ParseCodebaseURI(%q) = %v, want ErrInvalidCodebaseURI", bad, err)
		}
	}
	if _, err := NewCodebaseURI(sysGitHub, slugWidgets, "a/b", "golang:."); !errors.Is(err, ErrInvalidCodebaseURI) {
		t.Errorf("a revision with a slash must be refused, got %v", err)
	}
}

// ParseMust parses a URI the test built itself.
func ParseMust(t *testing.T, s string) CodebaseURI {
	t.Helper()
	u, err := ParseCodebaseURI(s)
	if err != nil {
		t.Fatalf("ParseCodebaseURI(%q): %v", s, err)
	}
	return u
}
