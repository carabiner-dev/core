// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// A project's members and the selectors over them (see members.proto). The
// functions here are the one implementation of naming an element and of
// selector matching, shared by every service: keystone derives a Member per
// placement from the edges, and whoever holds a MemberSelector evaluates it
// with Matches.

// MaxMemberSelectorValues is how many roles, relationship types or kinds one
// selector may list.
const MaxMemberSelectorValues = 32

var (
	// ErrInvalidElementRef marks a reference that names no element, or two.
	ErrInvalidElementRef = errors.New("invalid element reference")

	// ErrInvalidMemberSelector marks a selector that cannot be evaluated.
	ErrInvalidMemberSelector = errors.New("invalid member selector")

	// ErrInvalidFolderPath marks a folder path that is not "/" or
	// "/segment/segment".
	ErrInvalidFolderPath = errors.New("invalid folder path")
)

// ElementKeyProject is the Key of a reference to the project itself.
const ElementKeyProject = "project"

// Validate checks that the reference names exactly one element with a
// non-empty id.
func (r *ElementRef) Validate() error {
	switch ref := r.GetRef().(type) {
	case *ElementRef_Project:
		if !ref.Project {
			return fmt.Errorf("%w: project is set to false", ErrInvalidElementRef)
		}
	case *ElementRef_RepositoryId:
		if ref.RepositoryId == "" {
			return fmt.Errorf("%w: empty repository id", ErrInvalidElementRef)
		}
	case *ElementRef_CodebaseId:
		if ref.CodebaseId == "" {
			return fmt.Errorf("%w: empty codebase id", ErrInvalidElementRef)
		}
	default:
		return fmt.Errorf("%w: it names nothing", ErrInvalidElementRef)
	}
	return nil
}

// IsProject reports whether the reference is the project itself.
func (r *ElementRef) IsProject() bool {
	return r.GetProject()
}

// Kind returns the member kind a reference names, unspecified for the
// project or an unset reference.
func (r *ElementRef) Kind() MemberKind {
	switch r.GetRef().(type) {
	case *ElementRef_RepositoryId:
		return MemberKind_MEMBER_KIND_REPOSITORY
	case *ElementRef_CodebaseId:
		return MemberKind_MEMBER_KIND_CODEBASE
	default:
		return MemberKind_MEMBER_KIND_UNSPECIFIED
	}
}

// Key renders the reference as one string: "project", "repository:<id>" or
// "codebase:<id>". Empty for a reference that names nothing, so two valid
// references are the same element when their keys are equal.
func (r *ElementRef) Key() string {
	switch ref := r.GetRef().(type) {
	case *ElementRef_Project:
		if ref.Project {
			return ElementKeyProject
		}
	case *ElementRef_RepositoryId:
		if ref.RepositoryId != "" {
			return "repository:" + ref.RepositoryId
		}
	case *ElementRef_CodebaseId:
		if ref.CodebaseId != "" {
			return "codebase:" + ref.CodebaseId
		}
	}
	return ""
}

// Same reports whether two references name the same element. References
// that name nothing are the same as nothing, not each other.
func (r *ElementRef) Same(o *ElementRef) bool {
	key := r.Key()
	return key != "" && key == o.Key()
}

// Member is what a selector is evaluated against: one member of a project
// as the service that holds the graph derives it.
type Member struct {
	// Kind is what the member is.
	Kind MemberKind
	// Role is the member's place in the graph.
	Role MemberRole
	// ReachedBy are the relationship types by which a root reaches the
	// member, directly or through other members. Empty for a root and for
	// an unrelated member.
	ReachedBy []string
	// FolderPath is the path of the folder the member is placed in, "/" for
	// the root folder.
	FolderPath string
	// Labels are the member's effective labels.
	Labels map[string]string
}

// IsEmpty reports whether the selector has no field set. An empty selector
// is invalid and matches nothing.
func (s *MemberSelector) IsEmpty() bool {
	return len(s.GetRoles()) == 0 && len(s.GetRelationshipTypes()) == 0 &&
		s.GetFolderPathPrefix() == "" && len(s.GetKinds()) == 0 && s.GetLabels() == nil
}

// Validate checks that the selector can be evaluated: at least one field is
// set, every listed role, type and kind is one this code knows, the folder
// path is well formed and the label selector, when set, is valid.
func (s *MemberSelector) Validate() error {
	if s.IsEmpty() {
		return fmt.Errorf("%w: it has no requirements; an empty selector matches nothing", ErrInvalidMemberSelector)
	}
	for _, list := range []int{len(s.GetRoles()), len(s.GetRelationshipTypes()), len(s.GetKinds())} {
		if list > MaxMemberSelectorValues {
			return fmt.Errorf("%w: %d values in one list, at most %d are allowed", ErrInvalidMemberSelector, list, MaxMemberSelectorValues)
		}
	}
	for _, role := range s.GetRoles() {
		if role == MemberRole_MEMBER_ROLE_UNSPECIFIED || MemberRole_name[int32(role)] == "" {
			return fmt.Errorf("%w: unknown role %d", ErrInvalidMemberSelector, role)
		}
	}
	for _, typ := range s.GetRelationshipTypes() {
		if err := ValidateRelationshipType(typ); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidMemberSelector, err)
		}
	}
	if prefix := s.GetFolderPathPrefix(); prefix != "" {
		if err := ValidateFolderPath(prefix); err != nil {
			return fmt.Errorf("%w: folder path prefix: %w", ErrInvalidMemberSelector, err)
		}
	}
	for _, kind := range s.GetKinds() {
		if kind == MemberKind_MEMBER_KIND_UNSPECIFIED || MemberKind_name[int32(kind)] == "" {
			return fmt.Errorf("%w: unknown kind %d", ErrInvalidMemberSelector, kind)
		}
	}
	if s.GetLabels() != nil {
		if err := s.GetLabels().Validate(); err != nil {
			return fmt.Errorf("%w: labels: %w", ErrInvalidMemberSelector, err)
		}
	}
	return nil
}

// Matches reports whether a member satisfies the selector: every set field
// admits it. A nil or empty selector matches nothing, and so does one that
// does not validate, so a selector never selects more than it says.
func (s *MemberSelector) Matches(m Member) bool {
	if s.IsEmpty() || s.Validate() != nil {
		return false
	}
	if roles := s.GetRoles(); len(roles) > 0 && !slices.Contains(roles, m.Role) {
		return false
	}
	if types := s.GetRelationshipTypes(); len(types) > 0 && !slices.ContainsFunc(m.ReachedBy, func(t string) bool { return slices.Contains(types, t) }) {
		return false
	}
	if prefix := s.GetFolderPathPrefix(); prefix != "" && !FolderPathHasPrefix(m.FolderPath, prefix) {
		return false
	}
	if kinds := s.GetKinds(); len(kinds) > 0 && !slices.Contains(kinds, m.Kind) {
		return false
	}
	if s.GetLabels() != nil && !s.GetLabels().Matches(m.Labels) {
		return false
	}
	return true
}

// ValidateFolderPath checks a folder path as keystone renders one: "/" for
// the root, else "/" followed by non-empty segments joined by "/", with no
// trailing slash and no "." or ".." segment.
func ValidateFolderPath(path string) error {
	if path == "/" {
		return nil
	}
	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("%w: %q does not start with /", ErrInvalidFolderPath, path)
	}
	if strings.HasSuffix(path, "/") {
		return fmt.Errorf("%w: %q ends with /", ErrInvalidFolderPath, path)
	}
	for segment := range strings.SplitSeq(path[1:], "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("%w: %q has an empty, . or .. segment", ErrInvalidFolderPath, path)
		}
	}
	return nil
}

// FolderPathHasPrefix reports whether path is the folder at prefix or one
// below it: "/build/ci" is under "/build" and under "/", not under "/bui".
func FolderPathHasPrefix(path, prefix string) bool {
	if prefix == "/" {
		return strings.HasPrefix(path, "/")
	}
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}
