// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package core

import "strings"

// SubjectTypeName returns the camelCase name of a subject type (e.g.
// "pipelineRun", "revision"), matching the cdevents subject.type naming. It is
// the canonical string rendering of a subject type — reused for annotation
// values, event-type strings and rule manifests so they never drift.
func SubjectTypeName(t SubjectType) string {
	return enumToCamel(t.String(), "SUBJECT_TYPE_")
}

// PredicateName returns the camelCase name of a predicate (e.g. "queued",
// "created"). It is the canonical string rendering of a predicate.
func PredicateName(p Predicate) string {
	return enumToCamel(p.String(), "PREDICATE_")
}

// SubjectRoleName returns the camelCase name of a subject role (e.g. "onBranch",
// "inRepository"). It is the canonical string rendering of a subject role.
func SubjectRoleName(r SubjectRole) string {
	return enumToCamel(r.String(), "SUBJECT_ROLE_")
}

// enumToCamel renders a proto enum value name (e.g. "SUBJECT_ROLE_ON_BRANCH") as
// camelCase ("onBranch") after stripping prefix, so the string forms of our
// enums share the camelCase of the annotation keys (and match cdevents naming).
func enumToCamel(name, prefix string) string {
	parts := strings.Split(strings.ToLower(strings.TrimPrefix(name, prefix)), "_")
	for i := 1; i < len(parts); i++ {
		if parts[i] != "" {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}
