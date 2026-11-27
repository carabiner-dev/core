// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"slices"
)

// Edge is one typed relationship of a project's graph, as the service that
// stores the graph hands it to DeriveMemberRoles.
type Edge struct {
	From *ElementRef
	To   *ElementRef
	Type string
}

// MemberPosition is a member's derived place in the graph: its role and the
// relationship types by which it is reached. In a project with a root that
// is how a root reaches it (empty for a root, a dependent and an unrelated
// member); in one with no root, how any other member does, which also gives
// it its role.
type MemberPosition struct {
	Role      MemberRole
	ReachedBy []string
}

// DeriveMemberRoles computes every member's role from a project's edges.
// memberKeys are the keys (ElementRef.Key) of the placed members;
// implicitRoots are members that are roots without an edge (the sole member
// of a single-repository project). Edges whose member ends are not members
// are ignored. The rules, in order of precedence:
//
//   - root: the project describes it, or it is an implicit root;
//   - dependency: some path from a root reaches it through dependency edges
//     alone (dependsOn, hasOptionalDependency), transitively;
//   - supporting: a root reaches it, but every path uses at least one other
//     edge type (a test, documentation, configuration, tooling, a variant);
//   - dependent: it reaches a root through dependency edges, transitively;
//   - unrelated: none of the above.
//
// A project with no root (a group) has no deliverable to measure from, so
// there every member counts as where an edge may start: a member is a
// dependency when another member points at it with a dependency edge,
// supporting when members only point at it with other types, and unrelated
// when none points at it. It is reached by the types of those edges, so
// "reached by dependsOn" and the dependency role select what some member
// depends on. No member of such a project is a root or a dependent.
//
// The result has an entry for every member key.
func DeriveMemberRoles(memberKeys []string, edges []Edge, implicitRoots []string) map[string]MemberPosition {
	members := make(map[string]bool, len(memberKeys))
	for _, key := range memberKeys {
		if key != "" {
			members[key] = true
		}
	}

	// Adjacency among members, by edge type, forward and backward.
	forward := map[string][]memberLink{}
	backward := map[string][]memberLink{}
	roots := map[string]bool{}
	for _, key := range implicitRoots {
		if members[key] {
			roots[key] = true
		}
	}
	for _, e := range edges {
		if !IsProjectRelationshipType(e.Type) || e.From.Validate() != nil || e.To.Validate() != nil {
			continue
		}
		to := e.To.Key()
		if !members[to] {
			continue
		}
		if e.From.IsProject() {
			if e.Type == RelationshipDescribes {
				roots[to] = true
			}
			continue
		}
		from := e.From.Key()
		if !members[from] {
			continue
		}
		forward[from] = append(forward[from], memberLink{to, e.Type})
		backward[to] = append(backward[to], memberLink{from, e.Type})
	}

	positions := make(map[string]MemberPosition, len(members))
	for key := range members {
		positions[key] = MemberPosition{Role: MemberRole_MEMBER_ROLE_UNRELATED}
	}
	for key := range roots {
		positions[key] = MemberPosition{Role: MemberRole_MEMBER_ROLE_ROOT}
	}

	// walk visits what start reaches over the links admitted by follow,
	// never entering roots, and calls visit on each new node.
	walk := func(adjacent map[string][]memberLink, start map[string]bool, follow func(string) bool, visit func(string)) map[string]bool {
		seen := map[string]bool{}
		queue := make([]string, 0, len(start))
		for key := range start {
			queue = append(queue, key)
		}
		for len(queue) > 0 {
			node := queue[0]
			queue = queue[1:]
			for _, l := range adjacent[node] {
				if !follow(l.typ) || roots[l.to] || seen[l.to] {
					continue
				}
				seen[l.to] = true
				visit(l.to)
				queue = append(queue, l.to)
			}
		}
		return seen
	}
	anyType := func(string) bool { return true }

	// Dependencies: reached from a root through dependency edges alone.
	dependencies := walk(forward, roots, IsDependencyRelationshipType, func(key string) {
		positions[key] = MemberPosition{Role: MemberRole_MEMBER_ROLE_DEPENDENCY}
	})
	// Supporting: reached from a root or a dependency by any edge, and not
	// a dependency.
	reached := map[string]bool{}
	for key := range roots {
		reached[key] = true
	}
	for key := range dependencies {
		reached[key] = true
	}
	supporting := walk(forward, reached, anyType, func(key string) {
		if !dependencies[key] {
			positions[key] = MemberPosition{Role: MemberRole_MEMBER_ROLE_SUPPORTING}
		}
	})
	for key := range supporting {
		reached[key] = true
	}
	// Dependents: reach a root through dependency edges, and are nothing
	// above.
	walk(backward, roots, IsDependencyRelationshipType, func(key string) {
		if !reached[key] {
			positions[key] = MemberPosition{Role: MemberRole_MEMBER_ROLE_DEPENDENT}
		}
	})

	// ReachedBy: the types of the edges into a reached member from reached
	// members.
	for key := range reached {
		if roots[key] {
			continue
		}
		var types []string
		for _, l := range backward[key] {
			if reached[l.to] {
				types = append(types, l.typ)
			}
		}
		slices.Sort(types)
		pos := positions[key]
		pos.ReachedBy = slices.Compact(types)
		positions[key] = pos
	}

	// No root: nothing above reached anything, and the members take their
	// positions from the edges among them.
	if len(roots) == 0 {
		positionRootless(positions, backward)
	}
	return positions
}

// memberLink is one end of an edge between two members in an adjacency
// list: the member at the other end and the edge's type.
type memberLink struct{ to, typ string }

// positionRootless gives the members of a project with no root their
// positions. Every member is where an edge may start: a member is reached
// by the edges other members point at it with, a dependency when one of
// them is a dependency edge and supporting otherwise. One no member points
// at keeps the position it has, unrelated. backward lists, per member, the
// edges that arrive at it: a link's end is where the edge comes from.
func positionRootless(positions map[string]MemberPosition, backward map[string][]memberLink) {
	for key := range positions {
		var types []string
		dependency := false
		for _, l := range backward[key] {
			// An edge from a member to itself reaches no one.
			if l.to == key {
				continue
			}
			types = append(types, l.typ)
			dependency = dependency || IsDependencyRelationshipType(l.typ)
		}
		if len(types) == 0 {
			continue
		}
		slices.Sort(types)
		role := MemberRole_MEMBER_ROLE_SUPPORTING
		if dependency {
			role = MemberRole_MEMBER_ROLE_DEPENDENCY
		}
		positions[key] = MemberPosition{Role: role, ReachedBy: slices.Compact(types)}
	}
}
