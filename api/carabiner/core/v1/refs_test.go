// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"slices"
	"testing"
)

const runRef = "pipelineRun:1234"

func TestNewObjectRefNormalizes(t *testing.T) {
	r := NewObjectRef(SubjectType_SUBJECT_TYPE_REVISION, "  ABC123 ")
	if r.GetId() != "abc123" {
		t.Errorf("revision id = %q, want lowercased, trimmed abc123", r.GetId())
	}
	if got := NewObjectRef(SubjectType_SUBJECT_TYPE_TAG, " V0.2 ").GetId(); got != "V0.2" {
		t.Errorf("tag id = %q, want case preserved V0.2", got)
	}
	if NewObjectRef(SubjectType_SUBJECT_TYPE_TAG, "  ") != nil {
		t.Error("empty id should yield nil")
	}
	if NewObjectRef(SubjectType_SUBJECT_TYPE_UNSPECIFIED, "x") != nil {
		t.Error("unspecified type should yield nil")
	}
}

func TestCanonicalAndParseRoundTrip(t *testing.T) {
	cases := []struct {
		ref  *ObjectRef
		want string
	}{
		{NewObjectRef(SubjectType_SUBJECT_TYPE_REVISION, "2222"), "revision:2222"},
		{NewObjectRef(SubjectType_SUBJECT_TYPE_TAG, "v0.2"), "tag:v0.2"},
		{NewObjectRef(SubjectType_SUBJECT_TYPE_PIPELINE_RUN, "1234"), runRef},
		{NewObjectRef(SubjectType_SUBJECT_TYPE_REPOSITORY, "repository://github.com/acme/widgets"), "repository:repository://github.com/acme/widgets"},
		{NewObjectRef(SubjectType_SUBJECT_TYPE_CHANGE, "42"), "change:42"},
	}
	for _, c := range cases {
		if got := c.ref.Canonical(); got != c.want {
			t.Errorf("Canonical() = %q, want %q", got, c.want)
		}
		parsed, err := ParseObjectRef(c.want)
		if err != nil {
			t.Errorf("ParseObjectRef(%q) error: %v", c.want, err)
			continue
		}
		if parsed.GetType() != c.ref.GetType() || parsed.GetId() != c.ref.GetId() {
			t.Errorf("ParseObjectRef(%q) = %v, want %v", c.want, parsed, c.ref)
		}
	}
	if (*ObjectRef)(nil).Canonical() != "" {
		t.Error("nil ref should render empty")
	}
}

func TestParseObjectRefRejectsGarbage(t *testing.T) {
	for _, s := range []string{"", "revision", "revision:", ":abc", "release:0.2", "Revision:abc"} {
		if _, err := ParseObjectRef(s); err == nil {
			t.Errorf("ParseObjectRef(%q) accepted", s)
		}
	}
	// A repository URI contains colons of its own; only the first one splits.
	r, err := ParseObjectRef("repository:repository://github.com/acme/widgets")
	if err != nil {
		t.Fatalf("ParseObjectRef error: %v", err)
	}
	if r.GetId() != "repository://github.com/acme/widgets" {
		t.Errorf("id = %q, want the full URI", r.GetId())
	}
}

func TestEventObjectRefs(t *testing.T) {
	repoURI := "repository://github.com/acme/widgets"
	ev := &Event{
		Type:    &EventType{Subject: SubjectType_SUBJECT_TYPE_PIPELINE_RUN, Predicate: Predicate_PREDICATE_FINISHED},
		Subject: &Subject{Id: "1234", Source: repoURI},
		Related: []*RelatedSubject{
			{Role: SubjectRole_SUBJECT_ROLE_IN_REPOSITORY, Type: SubjectType_SUBJECT_TYPE_REPOSITORY, Descriptor_: &ResourceDescriptor{Uri: repoURI}},
			{Role: SubjectRole_SUBJECT_ROLE_HEAD, Type: SubjectType_SUBJECT_TYPE_REVISION, Descriptor_: &ResourceDescriptor{Digest: map[string]string{DigestGitCommit: "ABCDEF"}}},
			{Role: SubjectRole_SUBJECT_ROLE_ON_BRANCH, Type: SubjectType_SUBJECT_TYPE_BRANCH, Descriptor_: &ResourceDescriptor{Name: "main"}},
			// Duplicate of the head revision, spelled differently: collapses.
			{Role: SubjectRole_SUBJECT_ROLE_PARENT, Type: SubjectType_SUBJECT_TYPE_REVISION, Descriptor_: &ResourceDescriptor{Digest: map[string]string{DigestGitCommit: "abcdef"}}},
			// Identifies nothing: dropped.
			{Role: SubjectRole_SUBJECT_ROLE_TAGGED, Type: SubjectType_SUBJECT_TYPE_TAG, Descriptor_: &ResourceDescriptor{}},
		},
	}
	got := CanonicalRefs(ev.ObjectRefs())
	want := []string{
		"branch:main",
		runRef,
		"repository:" + repoURI,
		"revision:abcdef",
	}
	if !slices.Equal(got, want) {
		t.Errorf("ObjectRefs() = %v, want %v", got, want)
	}
	if ev.ObjectRef().Canonical() != runRef {
		t.Errorf("principal ref = %q", ev.ObjectRef().Canonical())
	}
	if (&Event{}).ObjectRef() != nil {
		t.Error("event without subject should have no principal ref")
	}
	if len((&Event{}).ObjectRefs()) != 0 {
		t.Error("empty event should have no refs")
	}
}

func TestRelatedSubjectObjectRefFallbacks(t *testing.T) {
	rev := &RelatedSubject{Type: SubjectType_SUBJECT_TYPE_REVISION, Descriptor_: &ResourceDescriptor{Name: "deadbeef"}}
	if got := rev.ObjectRef().Canonical(); got != "revision:deadbeef" {
		t.Errorf("revision without digest = %q, want name fallback", got)
	}
	repo := &RelatedSubject{Type: SubjectType_SUBJECT_TYPE_REPOSITORY, Descriptor_: &ResourceDescriptor{Name: "acme/widgets"}}
	if got := repo.ObjectRef().Canonical(); got != "repository:acme/widgets" {
		t.Errorf("repository without uri = %q, want name fallback", got)
	}
	br := &RelatedSubject{Type: SubjectType_SUBJECT_TYPE_BRANCH, Descriptor_: &ResourceDescriptor{Uri: "refs/heads/main"}}
	if got := br.ObjectRef().Canonical(); got != "branch:refs/heads/main" {
		t.Errorf("branch without name = %q, want uri fallback", got)
	}
}
