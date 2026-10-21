// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"slices"
	"testing"
)

const (
	fwBaseline = "baseline"
	fwCRA      = "cra"
	fwSLSA     = "slsa"
	repoMain   = "r-main"
	repoLib    = "r-lib"
	cbWeb      = "cb-web"
	tProject   = "project cra"
	tOthers    = "others baseline 1"
	tWebSLSA   = "web slsa"
)

// target is the resolver's view of a nomos ComplianceTarget.
type target struct {
	name      string
	anchor    *TargetAnchor
	members   *MemberSelector
	framework string
	replaces  []string
}

func (t *target) GetAnchor() *TargetAnchor    { return t.anchor }
func (t *target) GetMembers() *MemberSelector { return t.members }
func (t *target) GetFrameworkId() string      { return t.framework }
func (t *target) GetReplaces() []string       { return t.replaces }
func projectAnchor() *TargetAnchor {
	return &TargetAnchor{At: &TargetAnchor_Project{Project: true}}
}

func folderAnchor(path string) *TargetAnchor {
	return &TargetAnchor{At: &TargetAnchor_FolderPath{FolderPath: path}}
}

func repositoryAnchor(id string) *TargetAnchor {
	return &TargetAnchor{At: &TargetAnchor_RepositoryId{RepositoryId: id}}
}

func codebaseAnchor(id string) *TargetAnchor {
	return &TargetAnchor{At: &TargetAnchor_CodebaseId{CodebaseId: id}}
}

func names(ts []*target) []string {
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		out = append(out, t.name)
	}
	return out
}

var (
	mainRepo = &Member{ID: repoMain, Kind: MemberKind_MEMBER_KIND_REPOSITORY, Role: MemberRole_MEMBER_ROLE_ROOT, FolderPath: "/"}
	libRepo  = &Member{ID: repoLib, Kind: MemberKind_MEMBER_KIND_REPOSITORY, Role: MemberRole_MEMBER_ROLE_DEPENDENCY, FolderPath: "/libs/go"}
	webCode  = &Member{ID: cbWeb, Kind: MemberKind_MEMBER_KIND_CODEBASE, RepositoryID: repoMain, Role: MemberRole_MEMBER_ROLE_SUPPORTING, FolderPath: "/"}
)

func TestTargetAnchorValidateAndContains(t *testing.T) {
	for _, bad := range []*TargetAnchor{nil, {}, folderAnchor("build"), repositoryAnchor(""), codebaseAnchor(""), {At: &TargetAnchor_Project{}}} {
		if err := bad.Validate(); !errors.Is(err, ErrInvalidTargetAnchor) {
			t.Errorf("Validate(%v) = %v, want ErrInvalidTargetAnchor", bad, err)
		}
		if bad.Contains(mainRepo) {
			t.Errorf("%v contains a member", bad)
		}
	}
	cases := []struct {
		anchor *TargetAnchor
		member *Member
		want   bool
	}{
		{projectAnchor(), libRepo, true},
		{folderAnchor("/libs"), libRepo, true},
		{folderAnchor("/libs/go"), libRepo, true},
		{folderAnchor("/libs"), mainRepo, false},
		{folderAnchor("/"), webCode, true},
		{repositoryAnchor(repoMain), mainRepo, true},
		{repositoryAnchor(repoMain), webCode, true},
		{repositoryAnchor(repoMain), libRepo, false},
		{codebaseAnchor(cbWeb), webCode, true},
		{codebaseAnchor(cbWeb), mainRepo, false},
	}
	for _, c := range cases {
		if got := c.anchor.Contains(c.member); got != c.want {
			t.Errorf("%s contains %s = %v, want %v", c.anchor.Key(), c.member.ID, got, c.want)
		}
	}
	if k := folderAnchor("/libs").Key(); k != "folder:/libs" {
		t.Errorf("Key = %q", k)
	}
}

// The design's examples: set once on the project, inherited below, the
// nearest target for a framework wins, and replaces widens what it stands
// in for.
func TestEffectiveTargets(t *testing.T) {
	roots := &MemberSelector{Roles: []MemberRole{MemberRole_MEMBER_ROLE_ROOT}}
	others := &MemberSelector{Roles: []MemberRole{MemberRole_MEMBER_ROLE_DEPENDENCY, MemberRole_MEMBER_ROLE_SUPPORTING}}
	projectCRA := &target{name: tProject, anchor: projectAnchor(), framework: fwCRA}
	rootBaseline := &target{name: "root baseline 3", anchor: projectAnchor(), members: roots, framework: fwBaseline}
	otherBaseline := &target{name: tOthers, anchor: projectAnchor(), members: others, framework: fwBaseline}
	libsFolderCRA := &target{name: "libs cra", anchor: folderAnchor("/libs"), framework: fwCRA}
	webBaseline := &target{name: "web baseline", anchor: codebaseAnchor(cbWeb), framework: fwBaseline}

	cases := []struct {
		name    string
		member  *Member
		targets []*target
		want    []string
	}{
		{"project defaults by role", mainRepo, []*target{projectCRA, rootBaseline, otherBaseline}, []string{tProject, "root baseline 3"}},
		{"a sibling at one anchor never closes another", libRepo, []*target{projectCRA, rootBaseline, otherBaseline}, []string{tProject, tOthers}},
		{"nearest folder wins for its framework, the rest inherits", libRepo, []*target{projectCRA, otherBaseline, libsFolderCRA}, []string{"libs cra", tOthers}},
		{"a codebase's own target replaces the inherited one of that framework", webCode, []*target{projectCRA, otherBaseline, webBaseline}, []string{"web baseline", tProject}},
		{"slsa plus the inherited cra", webCode, []*target{projectCRA, {name: tWebSLSA, anchor: codebaseAnchor(cbWeb), framework: fwSLSA}}, []string{tWebSLSA, tProject}},
		{"slsa instead of the inherited cra", webCode, []*target{projectCRA, otherBaseline, {name: tWebSLSA, anchor: codebaseAnchor(cbWeb), framework: fwSLSA, replaces: []string{fwCRA}}}, []string{tWebSLSA, tOthers}},
		{"the whole story", webCode, []*target{projectCRA, otherBaseline, {name: tWebSLSA, anchor: codebaseAnchor(cbWeb), framework: fwSLSA, replaces: []string{ReplacesAll}}, {name: "web docs", anchor: codebaseAnchor(cbWeb), framework: "docs"}}, []string{tWebSLSA, "web docs"}},
		{"a repository target reaches its codebase", webCode, []*target{projectCRA, {name: "main repo cra", anchor: repositoryAnchor(repoMain), framework: fwCRA}}, []string{"main repo cra"}},
		{"a selector narrows under the anchor", libRepo, []*target{{name: "libs codebases", anchor: folderAnchor("/libs"), members: &MemberSelector{Kinds: []MemberKind{MemberKind_MEMBER_KIND_CODEBASE}}, framework: fwCRA}, projectCRA}, []string{tProject}},
		{"nothing reaches", mainRepo, []*target{libsFolderCRA}, nil},
	}
	for _, c := range cases {
		got := names(EffectiveTargets(c.member, c.targets))
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}
