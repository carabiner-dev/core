// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Labels are key/value pairs on platform resources that rules select
// resources by (see labels.proto for the syntax). The functions here are the
// one implementation of that syntax, of label inheritance and of selector
// matching, shared by every service.

const (
	// MaxLabels is how many labels of its own a resource may carry.
	MaxLabels = 64

	// MaxSelectorRequirements is how many requirements a selector may have,
	// match_labels and match_expressions together.
	MaxSelectorRequirements = 32

	// MaxRequirementValues is how many values one requirement may list.
	MaxRequirementValues = 64

	// ReservedLabelDomain is the key prefix, with its subdomains, reserved for
	// labels the platform sets itself.
	ReservedLabelDomain = "carabiner.dev"

	maxLabelNameLength   = 63
	maxLabelPrefixLength = 253
)

var (
	// ErrInvalidLabel marks a label set, key or value that breaks the syntax
	// or the limits.
	ErrInvalidLabel = errors.New("invalid label")

	// ErrInvalidSelector marks a selector that cannot be evaluated.
	ErrInvalidSelector = errors.New("invalid label selector")
)

// ValidateLabels checks a resource's own labels: their number and the syntax
// of every key and value. It does not refuse reserved keys, which the
// platform itself may set; see IsReservedLabelKey.
func ValidateLabels(labels map[string]string) error {
	if len(labels) > MaxLabels {
		return fmt.Errorf("%w: %d labels, at most %d are allowed", ErrInvalidLabel, len(labels), MaxLabels)
	}
	// In key order, so the same set always reports the same problem.
	for _, key := range SortedLabelKeys(labels) {
		if err := ValidateLabelKey(key); err != nil {
			return err
		}
		if err := ValidateLabelValue(labels[key]); err != nil {
			return fmt.Errorf("label %q: %w", key, err)
		}
	}
	return nil
}

// ValidateLabelKey checks a label key: an optional DNS subdomain prefix and a
// name, "prefix/name".
func ValidateLabelKey(key string) error {
	prefix, name, prefixed := strings.Cut(key, "/")
	if !prefixed {
		prefix, name = "", key
	}
	if !isLabelName(name) {
		return fmt.Errorf(
			"%w: key %q: the name must be 1 to %d characters of [A-Za-z0-9_.-], starting and ending with a letter or digit",
			ErrInvalidLabel, key, maxLabelNameLength,
		)
	}
	if prefixed && !isLabelPrefix(prefix) {
		return fmt.Errorf(
			"%w: key %q: the prefix must be a lowercase DNS subdomain of at most %d characters",
			ErrInvalidLabel, key, maxLabelPrefixLength,
		)
	}
	return nil
}

// ValidateLabelValue checks a label value: empty, or with the syntax of a key
// name.
func ValidateLabelValue(value string) error {
	if value == "" || isLabelName(value) {
		return nil
	}
	return fmt.Errorf(
		"%w: value %q: it must be empty or up to %d characters of [A-Za-z0-9_.-], starting and ending with a letter or digit",
		ErrInvalidLabel, value, maxLabelNameLength,
	)
}

// IsReservedLabelKey reports whether a key is under the prefix reserved for
// labels the platform sets itself ("carabiner.dev/…" or a subdomain of it).
// Users may select on such labels but never set or remove them.
func IsReservedLabelKey(key string) bool {
	prefix, _, prefixed := strings.Cut(key, "/")
	if !prefixed {
		return false
	}
	return prefix == ReservedLabelDomain || strings.HasSuffix(prefix, "."+ReservedLabelDomain)
}

// EffectiveLabels returns the labels selectors match on a resource that
// inherits: its own labels with the inherited ones laid over them. On a key
// both set the inherited value wins, which is what locks a key set on a
// namespace on its repositories. The result is a new map, never nil.
func EffectiveLabels(own, inherited map[string]string) map[string]string {
	effective := make(map[string]string, len(own)+len(inherited))
	maps.Copy(effective, own)
	maps.Copy(effective, inherited)
	return effective
}

// ShadowedLabelKeys returns, sorted, the keys a resource sets itself that it
// also inherits. Its own values under those keys are inert while the
// inheritance lasts.
func ShadowedLabelKeys(own, inherited map[string]string) []string {
	shadowed := []string{}
	for key := range own {
		if _, ok := inherited[key]; ok {
			shadowed = append(shadowed, key)
		}
	}
	slices.Sort(shadowed)
	return shadowed
}

// SortedLabelKeys returns a label set's keys, sorted. Never nil.
func SortedLabelKeys(labels map[string]string) []string {
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// IsEmpty reports whether the selector has no requirement at all. An empty
// selector is invalid and matches nothing.
func (s *LabelSelector) IsEmpty() bool {
	return len(s.GetMatchLabels()) == 0 && len(s.GetMatchExpressions()) == 0
}

// Validate checks that the selector can be evaluated: it has at least one
// requirement, its keys and values follow the label syntax, and each
// expression's values fit its operator.
func (s *LabelSelector) Validate() error {
	if s.IsEmpty() {
		return fmt.Errorf("%w: it has no requirements; an empty selector matches nothing", ErrInvalidSelector)
	}
	if n := len(s.GetMatchLabels()) + len(s.GetMatchExpressions()); n > MaxSelectorRequirements {
		return fmt.Errorf("%w: %d requirements, at most %d are allowed", ErrInvalidSelector, n, MaxSelectorRequirements)
	}
	for _, key := range SortedLabelKeys(s.GetMatchLabels()) {
		if err := validateSelectorPair(key, s.GetMatchLabels()[key]); err != nil {
			return err
		}
	}
	for i, req := range s.GetMatchExpressions() {
		if err := req.validate(); err != nil {
			return fmt.Errorf("%w: expression %d: %w", ErrInvalidSelector, i, err)
		}
	}
	return nil
}

func validateSelectorPair(key, value string) error {
	if err := ValidateLabelKey(key); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidSelector, err)
	}
	if err := ValidateLabelValue(value); err != nil {
		return fmt.Errorf("%w: label %q: %w", ErrInvalidSelector, key, err)
	}
	return nil
}

func (r *LabelRequirement) validate() error {
	if err := ValidateLabelKey(r.GetKey()); err != nil {
		return err
	}
	values := r.GetValues()
	switch r.GetOperator() {
	case LabelOperator_LABEL_OPERATOR_IN, LabelOperator_LABEL_OPERATOR_NOT_IN:
		if len(values) == 0 {
			return fmt.Errorf("key %q: the operator needs at least one value", r.GetKey())
		}
		if len(values) > MaxRequirementValues {
			return fmt.Errorf("key %q: %d values, at most %d are allowed", r.GetKey(), len(values), MaxRequirementValues)
		}
		for _, value := range values {
			if err := ValidateLabelValue(value); err != nil {
				return fmt.Errorf("key %q: %w", r.GetKey(), err)
			}
		}
	case LabelOperator_LABEL_OPERATOR_EXISTS, LabelOperator_LABEL_OPERATOR_DOES_NOT_EXIST:
		if len(values) != 0 {
			return fmt.Errorf("key %q: the operator takes no values", r.GetKey())
		}
	case LabelOperator_LABEL_OPERATOR_UNSPECIFIED:
		return fmt.Errorf("key %q: no operator", r.GetKey())
	default:
		return fmt.Errorf("key %q: unknown operator %d", r.GetKey(), r.GetOperator())
	}
	return nil
}

// Matches reports whether a resource's effective labels satisfy the selector:
// every match_labels pair and every expression holds. A nil or empty selector
// matches nothing, and so does a requirement with an operator this code does
// not know, so a selector never selects more than it says.
func (s *LabelSelector) Matches(labels map[string]string) bool {
	if s.IsEmpty() {
		return false
	}
	for key, want := range s.GetMatchLabels() {
		if got, ok := labels[key]; !ok || got != want {
			return false
		}
	}
	for _, req := range s.GetMatchExpressions() {
		if !req.matches(labels) {
			return false
		}
	}
	return true
}

func (r *LabelRequirement) matches(labels map[string]string) bool {
	value, set := labels[r.GetKey()]
	switch r.GetOperator() {
	case LabelOperator_LABEL_OPERATOR_IN:
		return set && slices.Contains(r.GetValues(), value)
	case LabelOperator_LABEL_OPERATOR_NOT_IN:
		return !set || !slices.Contains(r.GetValues(), value)
	case LabelOperator_LABEL_OPERATOR_EXISTS:
		return set
	case LabelOperator_LABEL_OPERATOR_DOES_NOT_EXIST:
		return !set
	case LabelOperator_LABEL_OPERATOR_UNSPECIFIED:
		return false
	default:
		return false
	}
}

// isLabelName reports whether s is a key name (or a non-empty value): 1 to
// 63 characters of [A-Za-z0-9_.-], starting and ending alphanumeric.
func isLabelName(s string) bool {
	if s == "" || len(s) > maxLabelNameLength {
		return false
	}
	if !isAlphanumeric(s[0]) || !isAlphanumeric(s[len(s)-1]) {
		return false
	}
	for i := range len(s) {
		if c := s[i]; !isAlphanumeric(c) && c != '_' && c != '.' && c != '-' {
			return false
		}
	}
	return true
}

// isLabelPrefix reports whether s is a lowercase DNS subdomain: dot-separated
// labels of [a-z0-9-], each starting and ending alphanumeric, at most 253
// characters in all.
func isLabelPrefix(s string) bool {
	if s == "" || len(s) > maxLabelPrefixLength {
		return false
	}
	for part := range strings.SplitSeq(s, ".") {
		if part == "" || len(part) > maxLabelNameLength {
			return false
		}
		if !isLowerAlphanumeric(part[0]) || !isLowerAlphanumeric(part[len(part)-1]) {
			return false
		}
		for i := range len(part) {
			if c := part[i]; !isLowerAlphanumeric(c) && c != '-' {
				return false
			}
		}
	}
	return true
}

func isLowerAlphanumeric(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
}

func isAlphanumeric(c byte) bool {
	return isLowerAlphanumeric(c) || (c >= 'A' && c <= 'Z')
}
