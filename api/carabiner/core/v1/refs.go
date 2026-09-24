// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"fmt"
	"slices"
	"strings"
)

// DigestGitCommit is the digest key under which a resource descriptor carries
// a git commit sha.
const DigestGitCommit = "gitCommit"

// refSeparator separates the subject type from the id in a canonical
// reference string.
const refSeparator = ":"

// NewObjectRef builds a reference to the object of kind t identified by id,
// normalized: surrounding whitespace is trimmed and revision ids are
// lowercased. It returns nil when the kind is unspecified or the id is empty,
// so callers can append the result unconditionally and drop nils.
func NewObjectRef(t SubjectType, id string) *ObjectRef {
	id = strings.TrimSpace(id)
	if id == "" || t == SubjectType_SUBJECT_TYPE_UNSPECIFIED {
		return nil
	}
	if t == SubjectType_SUBJECT_TYPE_REVISION {
		id = strings.ToLower(id)
	}
	return &ObjectRef{Type: t, Id: id}
}

// Canonical renders the reference in its canonical string form,
// "<subjectType>:<id>" (e.g. "revision:2222…"). A nil or incomplete reference
// renders as "".
func (r *ObjectRef) Canonical() string {
	if r == nil || r.GetId() == "" || r.GetType() == SubjectType_SUBJECT_TYPE_UNSPECIFIED {
		return ""
	}
	return SubjectTypeName(r.GetType()) + refSeparator + r.GetId()
}

// ParseObjectRef parses a canonical reference string back into an ObjectRef,
// applying the same normalization as NewObjectRef.
func ParseObjectRef(s string) (*ObjectRef, error) {
	name, id, found := strings.Cut(strings.TrimSpace(s), refSeparator)
	if !found || name == "" || strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("invalid object reference %q: want <subjectType>:<id>", s)
	}
	t, ok := subjectTypeByName(name)
	if !ok {
		return nil, fmt.Errorf("invalid object reference %q: unknown subject type %q", s, name)
	}
	return NewObjectRef(t, id), nil
}

// subjectTypeByName resolves the camelCase name of a subject type (as
// SubjectTypeName renders it) back to the enum value.
func subjectTypeByName(name string) (SubjectType, bool) {
	for _, v := range SubjectType_value {
		t := SubjectType(v)
		if t != SubjectType_SUBJECT_TYPE_UNSPECIFIED && SubjectTypeName(t) == name {
			return t, true
		}
	}
	return SubjectType_SUBJECT_TYPE_UNSPECIFIED, false
}

// ObjectRef returns the reference of the event's principal subject: its
// type's subject kind and the subject's id. Nil when the event has no subject
// or no id.
func (e *Event) ObjectRef() *ObjectRef {
	if e.GetSubject() == nil {
		return nil
	}
	return NewObjectRef(e.GetType().GetSubject(), e.GetSubject().GetId())
}

// ObjectRef returns the reference of a related subject, identified from its
// descriptor by kind: a revision by its gitCommit digest, a repository by its
// URI, anything else by its name (with the URI as a fallback). Nil when the
// descriptor identifies nothing.
func (r *RelatedSubject) ObjectRef() *ObjectRef {
	d := r.GetDescriptor_()
	var candidates []string
	switch r.GetType() { //nolint:exhaustive // every other kind takes the default naming
	case SubjectType_SUBJECT_TYPE_REVISION:
		candidates = []string{d.GetDigest()[DigestGitCommit], d.GetName()}
	case SubjectType_SUBJECT_TYPE_REPOSITORY:
		candidates = []string{d.GetUri(), d.GetName()}
	default:
		candidates = []string{d.GetName(), d.GetUri()}
	}
	for _, id := range candidates {
		if id != "" {
			return NewObjectRef(r.GetType(), id)
		}
	}
	return nil
}

// ObjectRefs returns the references of every object the event is about: its
// principal subject and each related subject, without duplicates, sorted by
// canonical form. It is what a service indexes to answer "everything about
// commit X" or "about release 0.2".
func (e *Event) ObjectRefs() []*ObjectRef {
	byKey := map[string]*ObjectRef{}
	add := func(r *ObjectRef) {
		if key := r.Canonical(); key != "" {
			byKey[key] = r
		}
	}
	add(e.ObjectRef())
	for _, rel := range e.GetRelated() {
		add(rel.ObjectRef())
	}
	keys := slices.Sorted(func(yield func(string) bool) {
		for k := range byKey {
			if !yield(k) {
				return
			}
		}
	})
	out := make([]*ObjectRef, 0, len(keys))
	for _, k := range keys {
		out = append(out, byKey[k])
	}
	return out
}

// CanonicalRefs renders references in canonical form, in order, dropping any
// that render empty.
func CanonicalRefs(refs []*ObjectRef) []string {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		if key := r.Canonical(); key != "" {
			out = append(out, key)
		}
	}
	return out
}
