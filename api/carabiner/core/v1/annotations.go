// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"google.golang.org/protobuf/types/known/structpb"
)

// Annotation keys for event-subject resource descriptors. They live in the
// dev.carabiner.* extension namespace and travel in ResourceDescriptor's
// annotations. Role and subject type are already first-class fields on a
// RelatedSubject; the annotations preserve them (and the primary-revision
// marker, which has no first-class home) when a descriptor is lifted out of an
// event to become an attestation subject.
const (
	// AnnotationPrimaryRevision marks the descriptor that is THE revision an
	// event is associated with: the commit to snapshot and attest content for.
	// Events whose principal subject is not itself a revision (a pull request,
	// a pipeline run) carry several revisions; this marks the canonical one. Its
	// value is the boolean true.
	AnnotationPrimaryRevision = "dev.carabiner.primaryRevision"

	// AnnotationSubjectRole records the SubjectRole a subject had in its event.
	// Its value is the camelCase role name (e.g. "onBranch").
	AnnotationSubjectRole = "dev.carabiner.subjectRole"

	// AnnotationSubjectType records the SubjectType a subject had in its event.
	// Its value is the camelCase subject-type name (e.g. "revision").
	AnnotationSubjectType = "dev.carabiner.subjectType"
)

// MarkPrimaryRevision sets the primary-revision annotation on d, allocating the
// annotations struct on first use. It is a no-op when d is nil.
func MarkPrimaryRevision(d *ResourceDescriptor) {
	setAnnotation(d, AnnotationPrimaryRevision, structpb.NewBoolValue(true))
}

// IsPrimaryRevision reports whether d carries the primary-revision annotation.
func IsPrimaryRevision(d *ResourceDescriptor) bool {
	return d.GetAnnotations().GetFields()[AnnotationPrimaryRevision].GetBoolValue()
}

// AnnotateSubjectOrigin records on d the role and subject type it had in its
// event, so the provenance survives when d is lifted out of the event to become
// an attestation subject. It is a no-op when d is nil.
func AnnotateSubjectOrigin(d *ResourceDescriptor, role SubjectRole, subjectType SubjectType) {
	setAnnotation(d, AnnotationSubjectRole, structpb.NewStringValue(SubjectRoleName(role)))
	setAnnotation(d, AnnotationSubjectType, structpb.NewStringValue(SubjectTypeName(subjectType)))
}

// setAnnotation sets a single annotation field on d, allocating the annotations
// struct (and its field map) on first use. It is a no-op when d is nil.
func setAnnotation(d *ResourceDescriptor, key string, value *structpb.Value) {
	if d == nil {
		return
	}
	switch {
	case d.Annotations == nil:
		d.Annotations = &structpb.Struct{Fields: map[string]*structpb.Value{}}
	case d.Annotations.Fields == nil:
		d.Annotations.Fields = map[string]*structpb.Value{}
	}
	d.Annotations.Fields[key] = value
}
