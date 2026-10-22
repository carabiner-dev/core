// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"fmt"
	"slices"
)

// The relationship types a project's edges may carry are a subset of SPDX
// 3.0.1's RelationshipType vocabulary, spelled as SPDX spells them, so a
// project's composition renders as an SPDX document and an SBOM can propose
// edges. A type is a string on the wire; this is the one list of the ones
// allowed, shared by keystone, which stores the edges, and everything that
// reads them.
const (
	// RelationshipDescribes marks a root: the project describes a member
	// that is a deliverable. Only the project is its source.
	RelationshipDescribes = "describes"
	// RelationshipDependsOn: the source needs the target to build or run.
	RelationshipDependsOn = "dependsOn"
	// RelationshipHasOptionalDependency: the source may use the target.
	RelationshipHasOptionalDependency = "hasOptionalDependency"
	// RelationshipHasTest: the target tests the source.
	RelationshipHasTest = "hasTest"
	// RelationshipHasDocumentation: the target documents the source.
	RelationshipHasDocumentation = "hasDocumentation"
	// RelationshipHasExample: the target shows how the source is used.
	RelationshipHasExample = "hasExample"
	// RelationshipConfigures: the source configures the target (infra and
	// policy repositories).
	RelationshipConfigures = "configures"
	// RelationshipPackagedBy: the target packages the source.
	RelationshipPackagedBy = "packagedBy"
	// RelationshipUsesTool: the source is built or checked with the target.
	RelationshipUsesTool = "usesTool"
	// RelationshipHasVariant: the target is a variant of the source (a fork,
	// a platform build).
	RelationshipHasVariant = "hasVariant"
	// RelationshipDescendantOf: the source descends from the target (moved
	// or forked code).
	RelationshipDescendantOf = "descendantOf"
	// RelationshipComponent: the target is a component of the source, a part
	// the source is assembled from that lives in its own repository or
	// codebase (a plugin, a module, a service of a larger product). Our own
	// type: SPDX's contains is for one artifact inside another, which a
	// separate repository is not. A component is a supporting member.
	RelationshipComponent = "component"
	// RelationshipOther is anything else, said in the edge's comment.
	RelationshipOther = "other"
)

// ProjectRelationshipTypes lists the types a project edge may carry, sorted.
var ProjectRelationshipTypes = []string{
	RelationshipComponent,
	RelationshipConfigures,
	RelationshipDependsOn,
	RelationshipDescendantOf,
	RelationshipDescribes,
	RelationshipHasDocumentation,
	RelationshipHasExample,
	RelationshipHasOptionalDependency,
	RelationshipHasTest,
	RelationshipHasVariant,
	RelationshipOther,
	RelationshipPackagedBy,
	RelationshipUsesTool,
}

// DependencyRelationshipTypes are the types through which a root reaches its
// dependencies; any other type reaches a supporting member.
var DependencyRelationshipTypes = []string{RelationshipDependsOn, RelationshipHasOptionalDependency}

// ErrInvalidRelationship marks an edge a project may not carry.
var ErrInvalidRelationship = errors.New("invalid relationship")

// IsProjectRelationshipType reports whether a project edge may carry the type.
func IsProjectRelationshipType(typ string) bool {
	_, ok := slices.BinarySearch(ProjectRelationshipTypes, typ)
	return ok
}

// IsDependencyRelationshipType reports whether the type makes its target a
// dependency of its source.
func IsDependencyRelationshipType(typ string) bool {
	return slices.Contains(DependencyRelationshipTypes, typ)
}

// ValidateRelationshipType refuses a type a project edge may not carry.
func ValidateRelationshipType(typ string) error {
	if typ == "" {
		return fmt.Errorf("%w: no type", ErrInvalidRelationship)
	}
	if !IsProjectRelationshipType(typ) {
		return fmt.Errorf("%w: type %q is not one a project may carry (%v)", ErrInvalidRelationship, typ, ProjectRelationshipTypes)
	}
	return nil
}

// ValidateProjectRelationship checks an edge of a project: a type the project
// may carry, two valid and different ends, "describes" from the project to a
// member and every other type between two members, and a comment when the
// type is "other".
func ValidateProjectRelationship(from, to *ElementRef, typ, comment string) error {
	if err := ValidateRelationshipType(typ); err != nil {
		return err
	}
	if err := from.Validate(); err != nil {
		return fmt.Errorf("%w: from: %w", ErrInvalidRelationship, err)
	}
	if err := to.Validate(); err != nil {
		return fmt.Errorf("%w: to: %w", ErrInvalidRelationship, err)
	}
	if to.IsProject() {
		return fmt.Errorf("%w: the project is never the target of an edge", ErrInvalidRelationship)
	}
	if from.Same(to) {
		return fmt.Errorf("%w: %s relates to itself", ErrInvalidRelationship, from.Key())
	}
	switch {
	case typ == RelationshipDescribes && !from.IsProject():
		return fmt.Errorf("%w: only the project describes a member, not %s", ErrInvalidRelationship, from.Key())
	case typ != RelationshipDescribes && from.IsProject():
		return fmt.Errorf("%w: the project only describes members; %q is between members", ErrInvalidRelationship, typ)
	case typ == RelationshipOther && comment == "":
		return fmt.Errorf("%w: an %q edge needs a comment saying what it is", ErrInvalidRelationship, RelationshipOther)
	}
	return nil
}
