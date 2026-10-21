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
	for _, in := range []string{"golang:./backend", "golang:backend", "golang:backend/", "golang:./backend/."} {
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
