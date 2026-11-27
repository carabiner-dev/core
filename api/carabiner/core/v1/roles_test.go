// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"slices"
	"testing"
)

// Keys of the members the role tests use most.
const (
	keyMain = "repository:main"
	keyCLI  = "repository:cli"
	keyA    = "repository:a"
	keyB    = "repository:b"
	keyLib  = "repository:lib"
)

func edge(from, to *ElementRef, typ string) Edge { return Edge{From: from, To: to, Type: typ} }

// The project the design describes: a main repository the project
// describes, a library it depends on, which depends on another, a docs
// codebase it has, a CLI that depends on it, a test repository of the CLI,
// and a stray repository.
func TestDeriveMemberRoles(t *testing.T) {
	members := []string{keyMain, keyLib, "repository:lib2", "codebase:docs", keyCLI, "repository:clitest", "repository:stray", "repository:gone"}
	edges := []Edge{
		edge(projectRef(), repoRef(memberMain), RelationshipDescribes),
		edge(repoRef(memberMain), repoRef(memberLib), RelationshipDependsOn),
		edge(repoRef(memberLib), repoRef("lib2"), RelationshipHasOptionalDependency),
		edge(repoRef(memberMain), codebaseRef(memberDocs), RelationshipHasDocumentation),
		edge(repoRef(memberCLI), repoRef(memberMain), RelationshipDependsOn),
		edge(repoRef(memberCLI), repoRef("clitest"), RelationshipHasTest),
		// A docs codebase's own dependency is reached only through a
		// non-dependency edge: supporting, not a dependency.
		edge(codebaseRef(memberDocs), repoRef("gone"), RelationshipDependsOn),
		// Edges to or from non-members, and of unknown types, are ignored.
		edge(repoRef(memberMain), repoRef("outsider"), RelationshipDependsOn),
		edge(repoRef("outsider"), repoRef(memberStray), RelationshipDependsOn),
		edge(repoRef(memberMain), repoRef(memberStray), "contains"),
	}
	got := DeriveMemberRoles(members, edges, nil)

	want := map[string]MemberPosition{
		keyMain:              {Role: MemberRole_MEMBER_ROLE_ROOT},
		keyLib:               {Role: MemberRole_MEMBER_ROLE_DEPENDENCY, ReachedBy: []string{RelationshipDependsOn}},
		"repository:lib2":    {Role: MemberRole_MEMBER_ROLE_DEPENDENCY, ReachedBy: []string{RelationshipHasOptionalDependency}},
		"codebase:docs":      {Role: MemberRole_MEMBER_ROLE_SUPPORTING, ReachedBy: []string{RelationshipHasDocumentation}},
		"repository:gone":    {Role: MemberRole_MEMBER_ROLE_SUPPORTING, ReachedBy: []string{RelationshipDependsOn}},
		keyCLI:               {Role: MemberRole_MEMBER_ROLE_DEPENDENT},
		"repository:clitest": {Role: MemberRole_MEMBER_ROLE_UNRELATED},
		"repository:stray":   {Role: MemberRole_MEMBER_ROLE_UNRELATED},
	}
	if len(got) != len(want) {
		t.Errorf("got %d positions, want %d: %v", len(got), len(want), got)
	}
	for key, w := range want {
		g, ok := got[key]
		if !ok {
			t.Errorf("%s: no position", key)
			continue
		}
		if g.Role != w.Role || !slices.Equal(g.ReachedBy, w.ReachedBy) {
			t.Errorf("%s: %v reached by %v, want %v reached by %v", key, g.Role, g.ReachedBy, w.Role, w.ReachedBy)
		}
	}

	// A member reached through several edges lists each type once, sorted.
	multi := DeriveMemberRoles([]string{keyMain, keyLib}, []Edge{
		edge(projectRef(), repoRef(memberMain), RelationshipDescribes),
		edge(repoRef(memberMain), repoRef(memberLib), RelationshipHasTest),
		edge(repoRef(memberMain), repoRef(memberLib), RelationshipDependsOn),
		edge(repoRef(memberMain), repoRef(memberLib), RelationshipDependsOn),
	}, nil)
	if pos := multi[keyLib]; pos.Role != MemberRole_MEMBER_ROLE_DEPENDENCY || !slices.Equal(pos.ReachedBy, []string{RelationshipDependsOn, RelationshipHasTest}) {
		t.Errorf("lib = %v reached by %v, want dependency reached by dependsOn, hasTest", pos.Role, pos.ReachedBy)
	}
}

func TestDeriveMemberRolesRootsAndCycles(t *testing.T) {
	// Describing both ends of a dependency makes both roots; a root is a
	// root whatever reaches it.
	two := DeriveMemberRoles([]string{keyMain, keyCLI}, []Edge{
		edge(projectRef(), repoRef(memberMain), RelationshipDescribes),
		edge(projectRef(), repoRef(memberCLI), RelationshipDescribes),
		edge(repoRef(memberCLI), repoRef(memberMain), RelationshipDependsOn),
	}, nil)
	for _, key := range []string{keyMain, keyCLI} {
		if two[key].Role != MemberRole_MEMBER_ROLE_ROOT || len(two[key].ReachedBy) != 0 {
			t.Errorf("%s = %v, want a root reached by nothing", key, two[key])
		}
	}

	// An implicit root (the sole member of a single-repository project) is
	// a root with no edge; one that is not a member is ignored.
	single := DeriveMemberRoles([]string{"repository:only"}, nil, []string{"repository:only", "repository:absent"})
	if single["repository:only"].Role != MemberRole_MEMBER_ROLE_ROOT || len(single) != 1 {
		t.Errorf("single = %v, want only a root", single)
	}

	// A cycle among dependencies terminates and keeps them dependencies.
	cycle := DeriveMemberRoles([]string{keyMain, keyA, keyB}, []Edge{
		edge(projectRef(), repoRef(memberMain), RelationshipDescribes),
		edge(repoRef(memberMain), repoRef("a"), RelationshipDependsOn),
		edge(repoRef("a"), repoRef("b"), RelationshipDependsOn),
		edge(repoRef("b"), repoRef("a"), RelationshipDependsOn),
		edge(repoRef("b"), repoRef(memberMain), RelationshipDependsOn),
	}, nil)
	for _, key := range []string{keyA, keyB} {
		if cycle[key].Role != MemberRole_MEMBER_ROLE_DEPENDENCY {
			t.Errorf("%s = %v, want dependency", key, cycle[key].Role)
		}
	}
	if cycle[keyMain].Role != MemberRole_MEMBER_ROLE_ROOT {
		t.Error("the root stays a root inside a cycle")
	}

	// No roots: every member is unrelated, whatever the edges.
	none := DeriveMemberRoles([]string{keyA, keyB}, []Edge{
		edge(repoRef("a"), repoRef("b"), RelationshipDependsOn),
	}, nil)
	for key, pos := range none {
		if pos.Role != MemberRole_MEMBER_ROLE_UNRELATED {
			t.Errorf("%s = %v, want unrelated", key, pos.Role)
		}
	}
	// The edge is still there to select by: b is what a depends on.
	if got := none[keyB].ReachedBy; !slices.Equal(got, []string{RelationshipDependsOn}) {
		t.Errorf("b reached by %v, want dependsOn", got)
	}
	if got := none[keyA].ReachedBy; len(got) != 0 {
		t.Errorf("a reached by %v, want nothing: no member points at it", got)
	}
	if len(DeriveMemberRoles(nil, nil, nil)) != 0 {
		t.Error("no members, no positions")
	}
}

// A group has no root, so no member has a role; its members are reached by
// the edges among them. A selector on a relationship type then picks what
// some member relates to that way.
func TestDeriveMemberRolesWithoutRoots(t *testing.T) {
	const keyTests = "repository:tests"
	group := DeriveMemberRoles([]string{keyA, keyB, keyLib, keyTests, keyCLI}, []Edge{
		// a and b both depend on lib; a also depends on b.
		edge(repoRef("a"), repoRef("lib"), RelationshipDependsOn),
		edge(repoRef("b"), repoRef("lib"), RelationshipDependsOn),
		edge(repoRef("a"), repoRef("b"), RelationshipDependsOn),
		// lib is also an optional dependency of b, and tests test a.
		edge(repoRef("b"), repoRef("lib"), RelationshipHasOptionalDependency),
		edge(repoRef("a"), repoRef("tests"), RelationshipHasTest),
		// An edge from a member to itself reaches no one, and one from
		// outside the project is ignored.
		edge(repoRef(memberCLI), repoRef(memberCLI), RelationshipDependsOn),
		edge(repoRef("elsewhere"), repoRef(memberCLI), RelationshipDependsOn),
	}, nil)

	want := map[string][]string{
		keyA:     nil,
		keyB:     {RelationshipDependsOn},
		keyLib:   {RelationshipDependsOn, RelationshipHasOptionalDependency},
		keyTests: {RelationshipHasTest},
		keyCLI:   nil,
	}
	for key, types := range want {
		pos := group[key]
		if pos.Role != MemberRole_MEMBER_ROLE_UNRELATED {
			t.Errorf("%s role = %v, want unrelated: a group has no roles", key, pos.Role)
		}
		if !slices.Equal(pos.ReachedBy, types) {
			t.Errorf("%s reached by %v, want %v", key, pos.ReachedBy, types)
		}
	}

	// What it means for a selector: by relationship it picks members, by
	// role it still picks none.
	byDependsOn := &MemberSelector{RelationshipTypes: []string{RelationshipDependsOn}}
	dependencies := &MemberSelector{Roles: []MemberRole{MemberRole_MEMBER_ROLE_DEPENDENCY}}
	var picked []string
	for _, key := range []string{keyA, keyB, keyLib, keyTests, keyCLI} {
		m := &Member{Role: group[key].Role, ReachedBy: group[key].ReachedBy}
		if byDependsOn.Matches(m) {
			picked = append(picked, key)
		}
		if dependencies.Matches(m) {
			t.Errorf("%s matched the dependency role in a group", key)
		}
	}
	if !slices.Equal(picked, []string{keyB, keyLib}) {
		t.Errorf("reached by dependsOn = %v, want b and lib", picked)
	}

	// With a root the rule is the old one: a member no root reaches is
	// reached by nothing, whatever other members point at it.
	rooted := DeriveMemberRoles([]string{keyMain, keyA, keyB}, []Edge{
		edge(projectRef(), repoRef(memberMain), RelationshipDescribes),
		edge(repoRef("a"), repoRef("b"), RelationshipDependsOn),
	}, nil)
	if pos := rooted[keyB]; pos.Role != MemberRole_MEMBER_ROLE_UNRELATED || len(pos.ReachedBy) != 0 {
		t.Errorf("b in a rooted project = %v, want unrelated and reached by nothing", pos)
	}
}
