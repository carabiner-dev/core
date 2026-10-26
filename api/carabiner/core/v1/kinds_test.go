// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"testing"
)

func TestEventKindsAreDistinctAndWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, k := range EventKinds() {
		if k.Subject == SubjectType_SUBJECT_TYPE_UNSPECIFIED || k.Predicate == Predicate_PREDICATE_UNSPECIFIED {
			t.Errorf("kind %v has an unspecified half", k)
		}
		key := SubjectTypeName(k.Subject) + "." + PredicateName(k.Predicate)
		if seen[key] {
			t.Errorf("kind %s listed twice", key)
		}
		seen[key] = true
		roles := map[SubjectRole]bool{}
		for _, r := range k.Related {
			if r.Role == SubjectRole_SUBJECT_ROLE_UNSPECIFIED || r.Type == SubjectType_SUBJECT_TYPE_UNSPECIFIED {
				t.Errorf("kind %s has an unspecified related subject %v", key, r)
			}
			if roles[r.Role] {
				t.Errorf("kind %s lists role %s twice", key, SubjectRoleName(r.Role))
			}
			roles[r.Role] = true
		}
		// Every kind happens in a repository.
		if !k.Yields(SubjectKind{Type: SubjectType_SUBJECT_TYPE_REPOSITORY, Role: SubjectRole_SUBJECT_ROLE_IN_REPOSITORY}) {
			t.Errorf("kind %s does not relate to its repository", key)
		}
	}
}

func TestLookupEventKindAndSubjects(t *testing.T) {
	kind, ok := LookupEventKind(&EventType{Subject: SubjectType_SUBJECT_TYPE_TAG, Predicate: Predicate_PREDICATE_CREATED})
	if !ok {
		t.Fatal("tag.created is not a kind")
	}
	want := []SubjectKind{
		{Type: SubjectType_SUBJECT_TYPE_TAG},
		{Type: SubjectType_SUBJECT_TYPE_REPOSITORY, Role: SubjectRole_SUBJECT_ROLE_IN_REPOSITORY},
		{Type: SubjectType_SUBJECT_TYPE_REVISION, Role: SubjectRole_SUBJECT_ROLE_TAGGED},
	}
	got := kind.Subjects()
	if len(got) != len(want) {
		t.Fatalf("subjects = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("subject %d = %v, want %v", i, got[i], want[i])
		}
	}
	// The principal is yielded in the unspecified role only; a pipeline run
	// is not a subject of a tag event.
	if !kind.Yields(SubjectKind{Type: SubjectType_SUBJECT_TYPE_TAG}) {
		t.Error("the principal tag is not yielded")
	}
	if kind.Yields(SubjectKind{Type: SubjectType_SUBJECT_TYPE_TAG, Role: SubjectRole_SUBJECT_ROLE_HEAD}) {
		t.Error("a tag in role head is yielded")
	}
	if kind.Yields(SubjectKind{Type: SubjectType_SUBJECT_TYPE_PIPELINE_RUN}) {
		t.Error("a pipeline run is yielded by a tag event")
	}
	// A merged change yields the merge revision; a created one does not.
	merge := SubjectKind{Type: SubjectType_SUBJECT_TYPE_REVISION, Role: SubjectRole_SUBJECT_ROLE_MERGE}
	merged, _ := LookupEventKind(&EventType{Subject: SubjectType_SUBJECT_TYPE_CHANGE, Predicate: Predicate_PREDICATE_MERGED})
	created, _ := LookupEventKind(&EventType{Subject: SubjectType_SUBJECT_TYPE_CHANGE, Predicate: Predicate_PREDICATE_CREATED})
	if !merged.Yields(merge) || created.Yields(merge) {
		t.Error("the merge revision is yielded by the wrong change kinds")
	}

	if _, ok := LookupEventKind(&EventType{Subject: SubjectType_SUBJECT_TYPE_REVISION, Predicate: Predicate_PREDICATE_MERGED}); ok {
		t.Error("revision.merged is a kind")
	}
	if _, ok := LookupEventKind(nil); ok {
		t.Error("a nil type is a kind")
	}
}

// widgetsRepoURI is the name of the repository the fixtures are about; its
// platform identity is widgetsRepoID.
const (
	widgetsRepoURI = "repository://github.com/acme/widgets"
	widgetsRepoID  = "1b2023ec-394f-4214-b516-df7724442e0d"
)

func TestArtifactKinds(t *testing.T) {
	published, ok := LookupEventKind(&EventType{Subject: SubjectType_SUBJECT_TYPE_ARTIFACT, Predicate: Predicate_PREDICATE_PUBLISHED})
	if !ok {
		t.Fatal("artifact.published is not a kind")
	}
	for _, s := range []SubjectKind{
		{Type: SubjectType_SUBJECT_TYPE_ARTIFACT},
		{Type: SubjectType_SUBJECT_TYPE_REPOSITORY, Role: SubjectRole_SUBJECT_ROLE_IN_REPOSITORY},
		{Type: SubjectType_SUBJECT_TYPE_REVISION, Role: SubjectRole_SUBJECT_ROLE_BUILT_FROM},
		{Type: SubjectType_SUBJECT_TYPE_TAG, Role: SubjectRole_SUBJECT_ROLE_RELEASED_AS},
	} {
		if !published.Yields(s) {
			t.Errorf("artifact.published does not yield %v", s)
		}
	}
	if published.Yields(SubjectKind{Type: SubjectType_SUBJECT_TYPE_PIPELINE_RUN, Role: SubjectRole_SUBJECT_ROLE_PRODUCED_BY}) {
		t.Error("artifact.published yields the producing run; only packaged does")
	}
	packaged, _ := LookupEventKind(&EventType{Subject: SubjectType_SUBJECT_TYPE_ARTIFACT, Predicate: Predicate_PREDICATE_PACKAGED})
	if !packaged.Yields(SubjectKind{Type: SubjectType_SUBJECT_TYPE_PIPELINE_RUN, Role: SubjectRole_SUBJECT_ROLE_PRODUCED_BY}) {
		t.Error("artifact.packaged does not yield the producing run")
	}

	// The names render as CDEvents spells them.
	if got := SubjectTypeName(SubjectType_SUBJECT_TYPE_ARTIFACT) + "." + PredicateName(Predicate_PREDICATE_PUBLISHED); got != "artifact.published" {
		t.Errorf("name = %q", got)
	}
	if got := SubjectRoleName(SubjectRole_SUBJECT_ROLE_RELEASED_AS); got != "releasedAs" {
		t.Errorf("role name = %q", got)
	}

	// An artifact carries its digest, or it is refused.
	event := &Event{
		Type:    published.Type(),
		Subject: &Subject{Id: "pkg:github/acme/widgets@v1.0.0#widgets-linux-amd64", Data: &Subject_Artifact{Artifact: &Artifact{Name: "widgets-linux-amd64", Kind: "binary"}}},
		Related: []*RelatedSubject{
			{Role: SubjectRole_SUBJECT_ROLE_IN_REPOSITORY, Type: SubjectType_SUBJECT_TYPE_REPOSITORY, Descriptor_: &ResourceDescriptor{Id: widgetsRepoID, Uri: widgetsRepoURI}},
			{Role: SubjectRole_SUBJECT_ROLE_RELEASED_AS, Type: SubjectType_SUBJECT_TYPE_TAG, Descriptor_: &ResourceDescriptor{Name: "v1.0.0"}},
		},
	}
	var missing *MissingDigestError
	if err := ValidateEvent(event); !errors.As(err, &missing) || missing.Error() != "event kind artifact.published: the artifact subject has no digest" {
		t.Errorf("artifact without digest: %v", err)
	}
	event.GetSubject().GetArtifact().Digest = map[string]string{"sha256": "0123"}
	if err := ValidateEvent(event); err != nil {
		t.Errorf("a well-formed artifact.published is refused: %v", err)
	}
	if got := event.ObjectRef().Canonical(); got != "artifact:pkg:github/acme/widgets@v1.0.0#widgets-linux-amd64" {
		t.Errorf("ref = %q", got)
	}
}

func TestValidateEvent(t *testing.T) {
	push := &Event{
		Type: &EventType{Subject: SubjectType_SUBJECT_TYPE_REVISION, Predicate: Predicate_PREDICATE_CREATED},
		Related: []*RelatedSubject{
			{Role: SubjectRole_SUBJECT_ROLE_IN_REPOSITORY, Type: SubjectType_SUBJECT_TYPE_REPOSITORY, Descriptor_: &ResourceDescriptor{Id: widgetsRepoID, Uri: widgetsRepoURI}},
			{Role: SubjectRole_SUBJECT_ROLE_ON_BRANCH, Type: SubjectType_SUBJECT_TYPE_BRANCH},
		},
	}
	if err := ValidateEvent(push); err != nil {
		t.Fatalf("a well-formed push is refused: %v", err)
	}
	// The repository must carry its platform identity; its name is not one.
	var missing *MissingIdentityError
	err := ValidateEvent(&Event{
		Type:    push.GetType(),
		Related: []*RelatedSubject{{Role: SubjectRole_SUBJECT_ROLE_IN_REPOSITORY, Type: SubjectType_SUBJECT_TYPE_REPOSITORY, Descriptor_: &ResourceDescriptor{Uri: widgetsRepoURI}}},
	})
	if !errors.As(err, &missing) || missing.Error() != "event kind revision.created: the repository subject in role inRepository has no platform identity (descriptor id)" {
		t.Errorf("repository without identity: %v", err)
	}
	// A related subject may be missing.
	if err := ValidateEvent(&Event{Type: push.GetType()}); err != nil {
		t.Fatalf("a push without related subjects is refused: %v", err)
	}

	var unknown *UnknownEventKindError
	err = ValidateEvent(&Event{Type: &EventType{Subject: SubjectType_SUBJECT_TYPE_BRANCH, Predicate: Predicate_PREDICATE_CREATED}})
	if !errors.As(err, &unknown) || unknown.Error() != "unknown event kind branch.created" {
		t.Errorf("unknown kind: %v", err)
	}

	var unexpected *UnexpectedSubjectError
	err = ValidateEvent(&Event{
		Type:    push.GetType(),
		Related: []*RelatedSubject{{Role: SubjectRole_SUBJECT_ROLE_HEAD, Type: SubjectType_SUBJECT_TYPE_REVISION}},
	})
	if !errors.As(err, &unexpected) || unexpected.Error() != "event kind revision.created does not yield a revision subject in role head" {
		t.Errorf("unexpected subject: %v", err)
	}
	// A related subject in the unspecified role is never valid.
	err = ValidateEvent(&Event{
		Type:    push.GetType(),
		Related: []*RelatedSubject{{Type: SubjectType_SUBJECT_TYPE_REPOSITORY}},
	})
	if !errors.As(err, &unexpected) {
		t.Errorf("unspecified role: %v", err)
	}
}
