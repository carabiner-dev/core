// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package core

import "slices"

// EventKind is one kind of event the platform defines: a subject and a
// predicate, with the neighborhood a producer attaches to it. The catalog of
// kinds is the contract between the system links that build events and the
// consumers that select subjects from them: a link emits an event of a kind
// with exactly the related subjects the kind lists, and a rule may only ask
// for subjects the kind yields.
type EventKind struct {
	Subject   SubjectType
	Predicate Predicate
	// Related are the related subjects an event of this kind carries, each a
	// role and the subject type found in it.
	Related []RelatedKind
}

// RelatedKind is one related subject of an event kind: the role it has
// relative to the principal subject and its type.
type RelatedKind struct {
	Role SubjectRole
	Type SubjectType
}

// SubjectKind is a subject an event kind can yield to an attestation: a type
// and the role it is found in, the unspecified role being the principal
// subject. It is what a rule's subject selector names.
type SubjectKind struct {
	Type SubjectType
	Role SubjectRole
}

// The neighborhoods the kinds share.
var (
	inRepository = RelatedKind{Role: SubjectRole_SUBJECT_ROLE_IN_REPOSITORY, Type: SubjectType_SUBJECT_TYPE_REPOSITORY}
	onBranch     = RelatedKind{Role: SubjectRole_SUBJECT_ROLE_ON_BRANCH, Type: SubjectType_SUBJECT_TYPE_BRANCH}
	headRevision = RelatedKind{Role: SubjectRole_SUBJECT_ROLE_HEAD, Type: SubjectType_SUBJECT_TYPE_REVISION}
	// A change relates to its head and base revisions and to the branches it
	// comes from and goes to.
	changeRelated = []RelatedKind{
		inRepository,
		headRevision,
		{Role: SubjectRole_SUBJECT_ROLE_BASE, Type: SubjectType_SUBJECT_TYPE_REVISION},
		{Role: SubjectRole_SUBJECT_ROLE_SOURCE_BRANCH, Type: SubjectType_SUBJECT_TYPE_BRANCH},
		{Role: SubjectRole_SUBJECT_ROLE_TARGET_BRANCH, Type: SubjectType_SUBJECT_TYPE_BRANCH},
	}
	// A run relates to the repository it ran in and the revision it ran at.
	runRelated = []RelatedKind{inRepository, headRevision}
)

// eventKinds is the catalog: every kind of event the platform defines, in a
// stable order (by subject, then by the lifecycle the predicates follow).
var eventKinds = []EventKind{
	// A commit pushed to a branch.
	{Subject: SubjectType_SUBJECT_TYPE_REVISION, Predicate: Predicate_PREDICATE_CREATED, Related: []RelatedKind{inRepository, onBranch}},
	// A tag created; the tagged revision rides along.
	{Subject: SubjectType_SUBJECT_TYPE_TAG, Predicate: Predicate_PREDICATE_CREATED, Related: []RelatedKind{
		inRepository,
		{Role: SubjectRole_SUBJECT_ROLE_TAGGED, Type: SubjectType_SUBJECT_TYPE_REVISION},
	}},
	// A pipeline run's lifecycle.
	{Subject: SubjectType_SUBJECT_TYPE_PIPELINE_RUN, Predicate: Predicate_PREDICATE_QUEUED, Related: runRelated},
	{Subject: SubjectType_SUBJECT_TYPE_PIPELINE_RUN, Predicate: Predicate_PREDICATE_STARTED, Related: runRelated},
	{Subject: SubjectType_SUBJECT_TYPE_PIPELINE_RUN, Predicate: Predicate_PREDICATE_FINISHED, Related: runRelated},
	// A change's lifecycle; a merged change also carries the merge revision.
	{Subject: SubjectType_SUBJECT_TYPE_CHANGE, Predicate: Predicate_PREDICATE_CREATED, Related: changeRelated},
	{Subject: SubjectType_SUBJECT_TYPE_CHANGE, Predicate: Predicate_PREDICATE_UPDATED, Related: changeRelated},
	{Subject: SubjectType_SUBJECT_TYPE_CHANGE, Predicate: Predicate_PREDICATE_REVIEWED, Related: changeRelated},
	{Subject: SubjectType_SUBJECT_TYPE_CHANGE, Predicate: Predicate_PREDICATE_MERGED, Related: append(slices.Clone(changeRelated),
		RelatedKind{Role: SubjectRole_SUBJECT_ROLE_MERGE, Type: SubjectType_SUBJECT_TYPE_REVISION},
	)},
	{Subject: SubjectType_SUBJECT_TYPE_CHANGE, Predicate: Predicate_PREDICATE_ABANDONED, Related: changeRelated},
}

// EventKinds returns the catalog of event kinds, in its stable order. The
// slice is a copy; the kinds' related lists are shared and must not be
// modified.
func EventKinds() []EventKind {
	return slices.Clone(eventKinds)
}

// LookupEventKind returns the kind of an event type, or false when the
// subject and predicate do not form a kind the platform defines.
func LookupEventKind(t *EventType) (EventKind, bool) {
	if t == nil {
		return EventKind{}, false
	}
	return lookupEventKind(t.GetSubject(), t.GetPredicate())
}

func lookupEventKind(subject SubjectType, predicate Predicate) (EventKind, bool) {
	for _, k := range eventKinds {
		if k.Subject == subject && k.Predicate == predicate {
			return k, true
		}
	}
	return EventKind{}, false
}

// Type returns the kind as an event type.
func (k EventKind) Type() *EventType {
	return &EventType{Subject: k.Subject, Predicate: k.Predicate}
}

// Subjects lists the subjects an event of this kind yields: its principal
// subject first (the kind's subject type, in the unspecified role), then
// every related subject.
func (k EventKind) Subjects() []SubjectKind {
	out := make([]SubjectKind, 0, 1+len(k.Related))
	out = append(out, SubjectKind{Type: k.Subject, Role: SubjectRole_SUBJECT_ROLE_UNSPECIFIED})
	for _, r := range k.Related {
		out = append(out, SubjectKind{Type: r.Type, Role: r.Role})
	}
	return out
}

// Yields reports whether an event of this kind yields the subject: the
// principal subject when the role is unspecified and the type is the kind's,
// else a related subject of that role and type.
func (k EventKind) Yields(s SubjectKind) bool {
	return slices.Contains(k.Subjects(), s)
}

// ValidateEvent checks an event against its kind: the type must be a kind the
// platform defines and every related subject must be one the kind lists. It
// does not require every listed related subject to be present, since a
// producer may lack one (a run without a head revision). A related
// repository must carry its platform identity (descriptor id): consumers
// route and authorize by it, never by the repository's name.
func ValidateEvent(e *Event) error {
	kind, ok := LookupEventKind(e.GetType())
	if !ok {
		return &UnknownEventKindError{Type: e.GetType()}
	}
	for _, rel := range e.GetRelated() {
		s := SubjectKind{Type: rel.GetType(), Role: rel.GetRole()}
		if s.Role == SubjectRole_SUBJECT_ROLE_UNSPECIFIED || !kind.Yields(s) {
			return &UnexpectedSubjectError{Kind: kind, Subject: s}
		}
		if s.Type == SubjectType_SUBJECT_TYPE_REPOSITORY && rel.GetDescriptor_().GetId() == "" {
			return &MissingIdentityError{Kind: kind, Subject: s}
		}
	}
	return nil
}

// MissingIdentityError says a related subject that must carry its platform
// identity (a repository) came without one.
type MissingIdentityError struct {
	Kind    EventKind
	Subject SubjectKind
}

func (e *MissingIdentityError) Error() string {
	return "event kind " + SubjectTypeName(e.Kind.Subject) + "." + PredicateName(e.Kind.Predicate) +
		": the " + SubjectTypeName(e.Subject.Type) + " subject in role " + SubjectRoleName(e.Subject.Role) + " has no platform identity (descriptor id)"
}

// UnknownEventKindError says an event type is not a kind the platform
// defines.
type UnknownEventKindError struct {
	Type *EventType
}

func (e *UnknownEventKindError) Error() string {
	return "unknown event kind " + SubjectTypeName(e.Type.GetSubject()) + "." + PredicateName(e.Type.GetPredicate())
}

// UnexpectedSubjectError says an event carries, or a selector asks for, a
// subject its kind does not yield.
type UnexpectedSubjectError struct {
	Kind    EventKind
	Subject SubjectKind
}

func (e *UnexpectedSubjectError) Error() string {
	return "event kind " + SubjectTypeName(e.Kind.Subject) + "." + PredicateName(e.Kind.Predicate) +
		" does not yield a " + SubjectTypeName(e.Subject.Type) + " subject in role " + SubjectRoleName(e.Subject.Role)
}
