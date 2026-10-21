// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"testing"
)

// The members of the project the selector tests run over, and the folder
// paths they sit in.
const (
	memberMain  = "main"
	memberLib   = "lib"
	memberCLI   = "cli"
	memberDocs  = "docs"
	memberStray = "stray"
	pathBuild   = "/build"
	pathBuildCI = "/build/ci"
	pathLibs    = "/libs"
)

func TestElementRef(t *testing.T) {
	cases := []struct {
		ref  *ElementRef
		key  string
		kind MemberKind
	}{
		{projectRef(), ElementKeyProject, MemberKind_MEMBER_KIND_UNSPECIFIED},
		{repoRef("r1"), "repository:r1", MemberKind_MEMBER_KIND_REPOSITORY},
		{codebaseRef("c1"), "codebase:c1", MemberKind_MEMBER_KIND_CODEBASE},
	}
	for _, c := range cases {
		if err := c.ref.Validate(); err != nil {
			t.Errorf("%s: %v", c.key, err)
		}
		if got := c.ref.Key(); got != c.key {
			t.Errorf("Key() = %q, want %q", got, c.key)
		}
		if got := c.ref.Kind(); got != c.kind {
			t.Errorf("%s: Kind() = %v, want %v", c.key, got, c.kind)
		}
		if !c.ref.Same(c.ref) {
			t.Errorf("%s must be the same as itself", c.key)
		}
	}
	if !projectRef().IsProject() || repoRef("r1").IsProject() {
		t.Error("IsProject must be true for the project alone")
	}
	if repoRef("r1").Same(codebaseRef("r1")) {
		t.Error("a repository and a codebase with the same id are different elements")
	}

	// References that name nothing: invalid, no key, the same as nothing.
	for name, ref := range map[string]*ElementRef{
		"nil ref": nil, "unset": {}, "project false": {Ref: &ElementRef_Project{Project: false}},
		"empty repository id": repoRef(""), "empty codebase id": codebaseRef(""),
	} {
		if err := ref.Validate(); !errors.Is(err, ErrInvalidElementRef) {
			t.Errorf("%s: Validate() = %v, want ErrInvalidElementRef", name, err)
		}
		if ref.Key() != "" {
			t.Errorf("%s: Key() = %q, want empty", name, ref.Key())
		}
		if ref.Same(ref) {
			t.Errorf("%s must not be the same as itself", name)
		}
	}
}

func TestValidateFolderPath(t *testing.T) {
	for _, p := range []string{"/", pathBuild, pathBuildCI, "/a-b_c.d/e"} {
		if err := ValidateFolderPath(p); err != nil {
			t.Errorf("%q must be valid: %v", p, err)
		}
	}
	for _, p := range []string{"", "build", pathBuild + "/", "//", "/build//ci", "/./ci", "/build/..", "./build"} {
		if err := ValidateFolderPath(p); !errors.Is(err, ErrInvalidFolderPath) {
			t.Errorf("%q must be invalid, got %v", p, err)
		}
	}
	under := []struct {
		path, prefix string
		want         bool
	}{
		{pathBuildCI, pathBuild, true},
		{pathBuild, pathBuild, true},
		{pathBuildCI, "/", true},
		{"/", "/", true},
		{"/builder", pathBuild, false},
		{"/", pathBuild, false},
		{"/ci/build", pathBuild, false},
	}
	for _, c := range under {
		if got := FolderPathHasPrefix(c.path, c.prefix); got != c.want {
			t.Errorf("FolderPathHasPrefix(%q, %q) = %v, want %v", c.path, c.prefix, got, c.want)
		}
	}
}

// The members of a project with a main repository, a library it depends on,
// a CLI that depends on it, a docs codebase it has, and a stray repository.
func projectMembers() map[string]Member {
	return map[string]Member{
		memberMain:  {Kind: MemberKind_MEMBER_KIND_REPOSITORY, Role: MemberRole_MEMBER_ROLE_ROOT, FolderPath: "/", Labels: map[string]string{keyTier: valProd}},
		memberLib:   {Kind: MemberKind_MEMBER_KIND_REPOSITORY, Role: MemberRole_MEMBER_ROLE_DEPENDENCY, ReachedBy: []string{RelationshipDependsOn}, FolderPath: pathLibs, Labels: map[string]string{keyTier: valProd}},
		memberCLI:   {Kind: MemberKind_MEMBER_KIND_REPOSITORY, Role: MemberRole_MEMBER_ROLE_ROOT, FolderPath: "/", Labels: map[string]string{keyTier: valDev}},
		memberDocs:  {Kind: MemberKind_MEMBER_KIND_CODEBASE, Role: MemberRole_MEMBER_ROLE_SUPPORTING, ReachedBy: []string{RelationshipHasDocumentation}, FolderPath: "/", Labels: map[string]string{CodebaseLanguageLabelKey: "markdown"}},
		memberStray: {Kind: MemberKind_MEMBER_KIND_REPOSITORY, Role: MemberRole_MEMBER_ROLE_UNRELATED, FolderPath: pathLibs + "/old", Labels: map[string]string{}},
	}
}

func TestMemberSelectorMatches(t *testing.T) {
	members := projectMembers()
	cases := []struct {
		name     string
		selector *MemberSelector
		want     []string
	}{
		{"roots", &MemberSelector{Roles: []MemberRole{MemberRole_MEMBER_ROLE_ROOT}}, []string{memberCLI, memberMain}},
		{"dependencies and supporting", &MemberSelector{Roles: []MemberRole{MemberRole_MEMBER_ROLE_DEPENDENCY, MemberRole_MEMBER_ROLE_SUPPORTING}}, []string{memberDocs, memberLib}},
		{"every role", &MemberSelector{Roles: []MemberRole{MemberRole_MEMBER_ROLE_ROOT, MemberRole_MEMBER_ROLE_DEPENDENCY, MemberRole_MEMBER_ROLE_SUPPORTING, MemberRole_MEMBER_ROLE_UNRELATED}}, []string{memberCLI, memberDocs, memberLib, memberMain, memberStray}},
		{"reached by a dependency edge", &MemberSelector{RelationshipTypes: []string{RelationshipDependsOn, RelationshipHasOptionalDependency}}, []string{memberLib}},
		{"under /libs", &MemberSelector{FolderPathPrefix: pathLibs}, []string{memberLib, memberStray}},
		{"everything under /", &MemberSelector{FolderPathPrefix: "/"}, []string{memberCLI, memberDocs, memberLib, memberMain, memberStray}},
		{"codebases", &MemberSelector{Kinds: []MemberKind{MemberKind_MEMBER_KIND_CODEBASE}}, []string{memberDocs}},
		{"by label", &MemberSelector{Labels: &LabelSelector{MatchLabels: map[string]string{keyTier: valProd}}}, []string{memberLib, memberMain}},
		{"by the language label", &MemberSelector{Labels: &LabelSelector{MatchLabels: map[string]string{CodebaseLanguageLabelKey: "markdown"}}}, []string{memberDocs}},
		{"fields combine", &MemberSelector{Roles: []MemberRole{MemberRole_MEMBER_ROLE_ROOT}, Labels: &LabelSelector{MatchLabels: map[string]string{keyTier: valProd}}}, []string{memberMain}},
		{"roots in /libs: none", &MemberSelector{Roles: []MemberRole{MemberRole_MEMBER_ROLE_ROOT}, FolderPathPrefix: pathLibs}, nil},
		{"nil selects nothing", nil, nil},
		{"empty selects nothing", &MemberSelector{}, nil},
		{"invalid selects nothing", &MemberSelector{RelationshipTypes: []string{notAProjectType}}, nil},
	}
	for _, c := range cases {
		var got []string
		for name, m := range members {
			if c.selector.Matches(m) {
				got = append(got, name)
			}
		}
		sortStrings(got)
		if !equalStrings(got, c.want) {
			t.Errorf("%s: matched %v, want %v", c.name, got, c.want)
		}
	}
}

func TestMemberSelectorValidate(t *testing.T) {
	valid := []*MemberSelector{
		{Roles: []MemberRole{MemberRole_MEMBER_ROLE_ROOT}},
		{RelationshipTypes: []string{RelationshipHasTest}},
		{FolderPathPrefix: "/"},
		{Kinds: []MemberKind{MemberKind_MEMBER_KIND_REPOSITORY, MemberKind_MEMBER_KIND_CODEBASE}},
		{Labels: &LabelSelector{MatchLabels: map[string]string{keyTier: valProd}}},
	}
	for i, s := range valid {
		if err := s.Validate(); err != nil {
			t.Errorf("valid selector %d: %v", i, err)
		}
	}
	tooMany := make([]MemberRole, MaxMemberSelectorValues+1)
	for i := range tooMany {
		tooMany[i] = MemberRole_MEMBER_ROLE_ROOT
	}
	invalid := map[string]*MemberSelector{
		"nil":                      nil,
		"empty":                    {},
		"unspecified role":         {Roles: []MemberRole{MemberRole_MEMBER_ROLE_UNSPECIFIED}},
		"unknown role":             {Roles: []MemberRole{MemberRole(99)}},
		"too many roles":           {Roles: tooMany},
		"unknown type":             {RelationshipTypes: []string{notAProjectType}},
		"bad folder path":          {FolderPathPrefix: "build/"},
		"unspecified kind":         {Kinds: []MemberKind{MemberKind_MEMBER_KIND_UNSPECIFIED}},
		"empty label selector":     {Labels: &LabelSelector{}},
		"invalid label selector":   {Labels: &LabelSelector{MatchLabels: map[string]string{keyBad: ""}}},
		"one bad field among good": {Roles: []MemberRole{MemberRole_MEMBER_ROLE_ROOT}, FolderPathPrefix: "nope"},
	}
	for name, s := range invalid {
		if err := s.Validate(); !errors.Is(err, ErrInvalidMemberSelector) {
			t.Errorf("%s: Validate() = %v, want ErrInvalidMemberSelector", name, err)
		}
	}
}
