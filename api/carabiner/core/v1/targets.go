// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// ErrInvalidTargetAnchor is returned by TargetAnchor.Validate.
var ErrInvalidTargetAnchor = errors.New("invalid target anchor")

// ReplacesAll is the value in a target's replaces list that closes every
// inherited framework: the target's anchor is the whole story.
const ReplacesAll = "*"

// Validate checks that the anchor names exactly one point and that a
// folder path is well formed.
func (a *TargetAnchor) Validate() error {
	switch at := a.GetAt().(type) {
	case *TargetAnchor_Project:
		if !at.Project {
			return fmt.Errorf("%w: project must be true when set", ErrInvalidTargetAnchor)
		}
	case *TargetAnchor_FolderPath:
		if err := ValidateFolderPath(at.FolderPath); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidTargetAnchor, err)
		}
	case *TargetAnchor_RepositoryId:
		if at.RepositoryId == "" {
			return fmt.Errorf("%w: empty repository id", ErrInvalidTargetAnchor)
		}
	case *TargetAnchor_CodebaseId:
		if at.CodebaseId == "" {
			return fmt.Errorf("%w: empty codebase id", ErrInvalidTargetAnchor)
		}
	case nil:
		return fmt.Errorf("%w: nothing set", ErrInvalidTargetAnchor)
	default:
		return fmt.Errorf("%w: unknown anchor", ErrInvalidTargetAnchor)
	}
	return nil
}

// Contains reports whether the member is under the anchor: every member is
// under the project, a member under the folders on its path, a repository
// and its codebases under the repository, a codebase under itself.
func (a *TargetAnchor) Contains(m *Member) bool {
	if a.Validate() != nil {
		return false
	}
	switch at := a.GetAt().(type) {
	case *TargetAnchor_Project:
		return true
	case *TargetAnchor_FolderPath:
		return FolderPathHasPrefix(m.FolderPath, at.FolderPath)
	case *TargetAnchor_RepositoryId:
		switch m.Kind {
		case MemberKind_MEMBER_KIND_REPOSITORY:
			return m.ID == at.RepositoryId
		case MemberKind_MEMBER_KIND_CODEBASE:
			return m.RepositoryID == at.RepositoryId
		case MemberKind_MEMBER_KIND_UNSPECIFIED:
			return false
		}
	case *TargetAnchor_CodebaseId:
		return m.Kind == MemberKind_MEMBER_KIND_CODEBASE && m.ID == at.CodebaseId
	}
	return false
}

// Key renders the anchor: "project", "folder:<path>", "repository:<id>"
// or "codebase:<id>"; empty for an invalid one.
func (a *TargetAnchor) Key() string {
	switch at := a.GetAt().(type) {
	case *TargetAnchor_Project:
		return "project"
	case *TargetAnchor_FolderPath:
		return "folder:" + at.FolderPath
	case *TargetAnchor_RepositoryId:
		return "repository:" + at.RepositoryId
	case *TargetAnchor_CodebaseId:
		return "codebase:" + at.CodebaseId
	}
	return ""
}

// distance ranks how near an anchor that contains the member is to it:
// the codebase itself, its repository, then the folders from the deepest
// up, then the project. Smaller is nearer.
func (a *TargetAnchor) distance(m *Member) int {
	const (
		self       = 0
		repository = 1
		folders    = 2
		project    = 1 << 30
	)
	switch at := a.GetAt().(type) {
	case *TargetAnchor_CodebaseId:
		return self
	case *TargetAnchor_RepositoryId:
		if m.Kind == MemberKind_MEMBER_KIND_REPOSITORY {
			return self
		}
		return repository
	case *TargetAnchor_FolderPath:
		return folders + folderDepth(m.FolderPath) - folderDepth(at.FolderPath)
	default:
		return project
	}
}

// folderDepth is the number of segments in a folder path: 0 for "/".
func folderDepth(path string) int {
	if path == "/" || path == "" {
		return 0
	}
	return strings.Count(path, "/")
}

// Target is a compliance target as the resolver needs it: where it is
// set, what it narrows to under there, the framework it asks for and the
// inherited frameworks it stands in for. nomos's ComplianceTarget
// satisfies it.
type Target interface {
	GetAnchor() *TargetAnchor
	GetMembers() *MemberSelector
	GetFrameworkId() string
	GetReplaces() []string
}

// TargetReaches reports whether a target applies to a member before
// inheritance is considered: its anchor contains the member and its
// selector, when set, matches it.
func TargetReaches(t Target, m *Member) bool {
	if !t.GetAnchor().Contains(m) {
		return false
	}
	return t.GetMembers() == nil || t.GetMembers().Matches(m)
}

// EffectiveTargets resolves the targets that apply to a member, nearest
// anchor first. Walking the member's anchors from the nearest out, the
// targets reaching it at an anchor apply unless their framework is
// closed; after the anchor, their frameworks close, and so do the ones
// they list in replaces (ReplacesAll closes everything and ends the walk).
// Every target at one anchor applies, whatever its siblings close.
func EffectiveTargets[T Target](m *Member, targets []T) []T {
	byDistance := map[int][]T{}
	for _, t := range targets {
		if TargetReaches(t, m) {
			d := t.GetAnchor().distance(m)
			byDistance[d] = append(byDistance[d], t)
		}
	}
	distances := make([]int, 0, len(byDistance))
	for d := range byDistance {
		distances = append(distances, d)
	}
	slices.Sort(distances)

	closed := map[string]bool{}
	var out []T
	for _, d := range distances {
		var closing []string
		all := false
		for _, t := range byDistance[d] {
			if closed[t.GetFrameworkId()] {
				continue
			}
			out = append(out, t)
			closing = append(closing, t.GetFrameworkId())
			for _, r := range t.GetReplaces() {
				if r == ReplacesAll {
					all = true
				} else {
					closing = append(closing, r)
				}
			}
		}
		for _, f := range closing {
			closed[f] = true
		}
		if all {
			break
		}
	}
	return out
}
