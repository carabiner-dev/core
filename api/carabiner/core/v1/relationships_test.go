// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"slices"
	"testing"
)

// notAProjectType is an SPDX relationship type a project may not carry.
const notAProjectType = "contains"

func projectRef() *ElementRef { return &ElementRef{Ref: &ElementRef_Project{Project: true}} }
func repoRef(id string) *ElementRef {
	return &ElementRef{Ref: &ElementRef_RepositoryId{RepositoryId: id}}
}

func codebaseRef(id string) *ElementRef {
	return &ElementRef{Ref: &ElementRef_CodebaseId{CodebaseId: id}}
}

// The allowed types are SPDX 3.0.1 values, spelled as spdx3's vocabulary
// spells them, so a project renders as an SPDX document without a mapping;
// component is the one type of our own (SPDX has none for a part that is a
// separate artifact).
func TestProjectRelationshipTypesAreSPDX(t *testing.T) {
	spdx := []string{
		"component", "configures", "dependsOn", "descendantOf", "describes", "hasDocumentation", "hasExample",
		"hasOptionalDependency", "hasTest", "hasVariant", "other", "packagedBy", "usesTool",
	}
	if !slices.Equal(ProjectRelationshipTypes, spdx) {
		t.Errorf("ProjectRelationshipTypes = %v, want the SPDX subset %v", ProjectRelationshipTypes, spdx)
	}
	if !slices.IsSorted(ProjectRelationshipTypes) {
		t.Error("ProjectRelationshipTypes must be sorted: IsProjectRelationshipType searches it")
	}
	for _, typ := range spdx {
		if !IsProjectRelationshipType(typ) {
			t.Errorf("%q must be a project relationship type", typ)
		}
	}
	for _, typ := range []string{"", notAProjectType, "DependsOn", "depends_on", "affects", "generates"} {
		if IsProjectRelationshipType(typ) {
			t.Errorf("%q must not be a project relationship type", typ)
		}
		if err := ValidateRelationshipType(typ); !errors.Is(err, ErrInvalidRelationship) {
			t.Errorf("ValidateRelationshipType(%q) = %v, want ErrInvalidRelationship", typ, err)
		}
	}
	for _, typ := range DependencyRelationshipTypes {
		if !IsDependencyRelationshipType(typ) || !IsProjectRelationshipType(typ) {
			t.Errorf("%q must be a dependency type and a project type", typ)
		}
	}
	if IsDependencyRelationshipType(RelationshipHasTest) {
		t.Error("hasTest reaches a supporting member, not a dependency")
	}
}

func TestValidateProjectRelationship(t *testing.T) {
	valid := []struct {
		name     string
		from, to *ElementRef
		typ      string
		comment  string
	}{
		{"project describes a repository", projectRef(), repoRef("r1"), RelationshipDescribes, ""},
		{"project describes a codebase", projectRef(), codebaseRef("c1"), RelationshipDescribes, ""},
		{"repository depends on a repository", repoRef("r1"), repoRef("r2"), RelationshipDependsOn, ""},
		{"codebase depends on a repository", codebaseRef("c1"), repoRef("r2"), RelationshipDependsOn, ""},
		{"repository has a test repository", repoRef("r1"), repoRef("r2"), RelationshipHasTest, ""},
		{"other with a comment", repoRef("r1"), repoRef("r2"), RelationshipOther, "shares a release train"},
		{"a component of the main repository", repoRef("r1"), codebaseRef("c1"), RelationshipComponent, ""},
	}
	for _, c := range valid {
		if err := ValidateProjectRelationship(c.from, c.to, c.typ, c.comment); err != nil {
			t.Errorf("%s: %v", c.name, err)
		}
	}

	invalid := []struct {
		name     string
		from, to *ElementRef
		typ      string
		comment  string
	}{
		{"unknown type", repoRef("r1"), repoRef("r2"), notAProjectType, ""},
		{"no type", repoRef("r1"), repoRef("r2"), "", ""},
		{"nil from", nil, repoRef("r2"), RelationshipDependsOn, ""},
		{"empty to", repoRef("r1"), &ElementRef{}, RelationshipDependsOn, ""},
		{"empty repository id", repoRef(""), repoRef("r2"), RelationshipDependsOn, ""},
		{"project as target", repoRef("r1"), projectRef(), RelationshipDependsOn, ""},
		{"project describes itself", projectRef(), projectRef(), RelationshipDescribes, ""},
		{"a member relates to itself", repoRef("r1"), repoRef("r1"), RelationshipDependsOn, ""},
		{"a member describes", repoRef("r1"), repoRef("r2"), RelationshipDescribes, ""},
		{"the project depends on a member", projectRef(), repoRef("r2"), RelationshipDependsOn, ""},
		{"other without a comment", repoRef("r1"), repoRef("r2"), RelationshipOther, ""},
	}
	for _, c := range invalid {
		err := ValidateProjectRelationship(c.from, c.to, c.typ, c.comment)
		if !errors.Is(err, ErrInvalidRelationship) {
			t.Errorf("%s: err = %v, want ErrInvalidRelationship", c.name, err)
		}
	}
}
