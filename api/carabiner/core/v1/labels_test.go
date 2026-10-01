// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

const (
	keyTier  = "tier"
	valProd  = "prod"
	valDev   = "dev"
	keyTeam  = "example.com/team"
	valInfra = "infra"
	// keyTag is used as a tag: a label with an empty value.
	keyTag = "experimental"
	// keyLang is a second plain key; the selector tests' resource lacks it.
	keyLang = "lang"
	// keyReserved is under the platform's reserved prefix.
	keyReserved = "carabiner.dev/visibility"
	// keyBad breaks the key syntax.
	keyBad = "ti er"
)

func TestValidateLabelKey(t *testing.T) {
	long := strings.Repeat("a", 63)
	valid := []string{
		keyTier, "a", "A1", "tier.v2", "tier_v2", "tier-v2", long,
		keyTeam, "a.b-c.example.com/Name", keyReserved,
		strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "/x",
	}
	for _, key := range valid {
		if err := ValidateLabelKey(key); err != nil {
			t.Errorf("%q must be valid: %v", key, err)
		}
	}
	invalid := []string{
		"", "-tier", "tier-", ".tier", "tier.", "_tier", keyBad, "tier=prod", "tíer", long + "a",
		"/name", "example.com/", "example.com/-x", "Example.com/x", "example..com/x", ".com/x",
		"-a.com/x", "a-.com/x", "a_b.com/x", "a/b/c", strings.Repeat("a", 64) + ".com/x",
		strings.Repeat("a.", 127) + "/x", strings.Repeat(strings.Repeat("a", 60)+".", 5) + "com/x",
	}
	for _, key := range invalid {
		err := ValidateLabelKey(key)
		if err == nil {
			t.Errorf("%q must be invalid", key)
			continue
		}
		if !errors.Is(err, ErrInvalidLabel) {
			t.Errorf("%q: error %v must wrap ErrInvalidLabel", key, err)
		}
	}
}

func TestValidateLabelValue(t *testing.T) {
	for _, value := range []string{"", valProd, "v1.2.3", "a_b-c", "A", strings.Repeat("a", 63)} {
		if err := ValidateLabelValue(value); err != nil {
			t.Errorf("%q must be valid: %v", value, err)
		}
	}
	for _, value := range []string{" ", "-prod", "prod-", "pro d", "a/b", "a=b", strings.Repeat("a", 64)} {
		if err := ValidateLabelValue(value); !errors.Is(err, ErrInvalidLabel) {
			t.Errorf("%q must be invalid, got %v", value, err)
		}
	}
}

func TestValidateLabels(t *testing.T) {
	if err := ValidateLabels(nil); err != nil {
		t.Errorf("no labels: %v", err)
	}
	// A tag (empty value), a dimension, a prefixed key and a reserved key,
	// which the platform itself may set.
	ok := map[string]string{keyTag: "", keyTier: valProd, keyTeam: valInfra, keyReserved: "public"}
	if err := ValidateLabels(ok); err != nil {
		t.Errorf("valid set: %v", err)
	}

	full := map[string]string{}
	for i := range MaxLabels {
		full[fmt.Sprintf("k%d", i)] = ""
	}
	if err := ValidateLabels(full); err != nil {
		t.Errorf("%d labels must be allowed: %v", MaxLabels, err)
	}
	full["one-more"] = ""
	if err := ValidateLabels(full); !errors.Is(err, ErrInvalidLabel) {
		t.Errorf("%d labels must be refused, got %v", len(full), err)
	}

	err := ValidateLabels(map[string]string{keyTier: "not valid"})
	if !errors.Is(err, ErrInvalidLabel) || !strings.Contains(err.Error(), `"tier"`) {
		t.Errorf("a bad value must name its key: %v", err)
	}
	// The same set always reports the same problem: the first bad key in order.
	bad := map[string]string{"z z": "", "a a": "", keyTier: valProd}
	for range 20 {
		if err := ValidateLabels(bad); err == nil || !strings.Contains(err.Error(), `"a a"`) {
			t.Fatalf("want the first bad key in key order, got %v", err)
		}
	}
}

func TestIsReservedLabelKey(t *testing.T) {
	for key, want := range map[string]bool{
		keyReserved:                   true,
		"sys.carabiner.dev/origin":    true,
		"a.b.carabiner.dev/x":         true,
		"carabiner.dev":               false, // a bare name, no prefix
		keyTier:                       false,
		keyTeam:                       false,
		"notcarabiner.dev/x":          false,
		"carabiner.dev.example.com/x": false,
		"xcarabiner.dev/x":            false,
	} {
		if got := IsReservedLabelKey(key); got != want {
			t.Errorf("IsReservedLabelKey(%q) = %v, want %v", key, got, want)
		}
	}
}

func TestEffectiveLabels(t *testing.T) {
	own := map[string]string{keyTier: valDev, keyLang: "go"}
	inherited := map[string]string{keyTier: valProd, keyTeam: valInfra}

	got := EffectiveLabels(own, inherited)
	want := map[string]string{keyTier: valProd, keyLang: "go", keyTeam: valInfra}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("effective = %v, want %v (the inherited value wins)", got, want)
	}
	// The inputs are left alone and the result is the caller's to change.
	got["new"] = ""
	if own[keyTier] != valDev || len(own) != 2 || len(inherited) != 2 {
		t.Errorf("inputs changed: own %v, inherited %v", own, inherited)
	}

	if shadowed := ShadowedLabelKeys(own, inherited); !slices.Equal(shadowed, []string{keyTier}) {
		t.Errorf("shadowed = %v, want [tier]", shadowed)
	}
	if shadowed := ShadowedLabelKeys(own, nil); shadowed == nil || len(shadowed) != 0 {
		t.Errorf("nothing inherited: shadowed = %#v, want an empty list", shadowed)
	}
	if got := EffectiveLabels(nil, nil); got == nil || len(got) != 0 {
		t.Errorf("no labels at all: %#v, want an empty map", got)
	}
	if keys := SortedLabelKeys(inherited); !slices.Equal(keys, []string{keyTeam, keyTier}) {
		t.Errorf("sorted keys = %v", keys)
	}
	if keys := SortedLabelKeys(nil); keys == nil || len(keys) != 0 {
		t.Errorf("sorted keys of nothing = %#v, want an empty list", keys)
	}
}

func requirement(key string, op LabelOperator, values ...string) *LabelRequirement {
	return &LabelRequirement{Key: key, Operator: op, Values: values}
}

func TestSelectorMatches(t *testing.T) {
	labels := map[string]string{keyTier: valProd, keyTag: "", keyTeam: valInfra}
	const (
		in       = LabelOperator_LABEL_OPERATOR_IN
		notIn    = LabelOperator_LABEL_OPERATOR_NOT_IN
		exists   = LabelOperator_LABEL_OPERATOR_EXISTS
		notExist = LabelOperator_LABEL_OPERATOR_DOES_NOT_EXIST
	)
	cases := []struct {
		name     string
		selector *LabelSelector
		want     bool
	}{
		{"a pair that is set", &LabelSelector{MatchLabels: map[string]string{keyTier: valProd}}, true},
		{"a pair with another value", &LabelSelector{MatchLabels: map[string]string{keyTier: valDev}}, false},
		{"a pair whose key is not set", &LabelSelector{MatchLabels: map[string]string{keyLang: "go"}}, false},
		{"a tag, by its empty value", &LabelSelector{MatchLabels: map[string]string{keyTag: ""}}, true},
		{"an empty value does not match a set value", &LabelSelector{MatchLabels: map[string]string{keyTier: ""}}, false},
		{"an empty value does not match a missing key", &LabelSelector{MatchLabels: map[string]string{keyLang: ""}}, false},
		{"all pairs must hold", &LabelSelector{MatchLabels: map[string]string{keyTier: valProd, keyTeam: "web"}}, false},
		{"all pairs hold", &LabelSelector{MatchLabels: map[string]string{keyTier: valProd, keyTeam: valInfra}}, true},

		{"in", &LabelSelector{MatchExpressions: []*LabelRequirement{requirement(keyTier, in, valDev, valProd)}}, true},
		{"in, other values", &LabelSelector{MatchExpressions: []*LabelRequirement{requirement(keyTier, in, valDev)}}, false},
		{"in, key not set", &LabelSelector{MatchExpressions: []*LabelRequirement{requirement(keyLang, in, "go")}}, false},
		{"not in, another value", &LabelSelector{MatchExpressions: []*LabelRequirement{requirement(keyTier, notIn, valDev)}}, true},
		{"not in, the value", &LabelSelector{MatchExpressions: []*LabelRequirement{requirement(keyTier, notIn, valProd)}}, false},
		{"not in, key not set", &LabelSelector{MatchExpressions: []*LabelRequirement{requirement(keyLang, notIn, "go")}}, true},
		{"exists", &LabelSelector{MatchExpressions: []*LabelRequirement{requirement(keyTier, exists)}}, true},
		{"exists, a tag", &LabelSelector{MatchExpressions: []*LabelRequirement{requirement(keyTag, exists)}}, true},
		{"exists, key not set", &LabelSelector{MatchExpressions: []*LabelRequirement{requirement(keyLang, exists)}}, false},
		{"does not exist", &LabelSelector{MatchExpressions: []*LabelRequirement{requirement(keyLang, notExist)}}, true},
		{"does not exist, but it does", &LabelSelector{MatchExpressions: []*LabelRequirement{requirement(keyTier, notExist)}}, false},

		{"pairs and expressions together", &LabelSelector{
			MatchLabels:      map[string]string{keyTier: valProd},
			MatchExpressions: []*LabelRequirement{requirement(keyTeam, in, valInfra), requirement("deprecated", notExist)},
		}, true},
		{"one expression fails", &LabelSelector{
			MatchLabels:      map[string]string{keyTier: valProd},
			MatchExpressions: []*LabelRequirement{requirement(keyTeam, in, valInfra), requirement(keyTag, notExist)},
		}, false},

		// A selector never selects more than it says.
		{"nil selector", nil, false},
		{"empty selector", &LabelSelector{}, false},
		{"no operator", &LabelSelector{MatchExpressions: []*LabelRequirement{requirement(keyTier, LabelOperator_LABEL_OPERATOR_UNSPECIFIED)}}, false},
		{"an operator from the future", &LabelSelector{MatchExpressions: []*LabelRequirement{requirement(keyTier, LabelOperator(99))}}, false},
	}
	for _, tc := range cases {
		if got := tc.selector.Matches(labels); got != tc.want {
			t.Errorf("%s: Matches = %v, want %v", tc.name, got, tc.want)
		}
	}

	// No labels at all: only absence can match.
	absent := &LabelSelector{MatchExpressions: []*LabelRequirement{requirement(keyTier, notExist)}}
	if !absent.Matches(nil) {
		t.Error("a resource without labels must match a does-not-exist requirement")
	}
	if (&LabelSelector{MatchLabels: map[string]string{keyTier: valProd}}).Matches(nil) {
		t.Error("a resource without labels must not match a pair")
	}
}

func TestSelectorValidate(t *testing.T) {
	const (
		in     = LabelOperator_LABEL_OPERATOR_IN
		exists = LabelOperator_LABEL_OPERATOR_EXISTS
	)
	valid := []*LabelSelector{
		{MatchLabels: map[string]string{keyTier: valProd}},
		{MatchLabels: map[string]string{keyTag: ""}},
		// Reserved keys can be selected on.
		{MatchLabels: map[string]string{keyReserved: "public"}},
		{MatchExpressions: []*LabelRequirement{requirement(keyTier, in, valProd, valDev, "")}},
		{MatchExpressions: []*LabelRequirement{requirement(keyTier, LabelOperator_LABEL_OPERATOR_NOT_IN, valDev)}},
		{MatchExpressions: []*LabelRequirement{requirement(keyTier, exists)}},
		{MatchExpressions: []*LabelRequirement{requirement(keyTier, LabelOperator_LABEL_OPERATOR_DOES_NOT_EXIST)}},
	}
	for i, s := range valid {
		if err := s.Validate(); err != nil {
			t.Errorf("valid selector %d: %v", i, err)
		}
	}

	many := &LabelSelector{MatchLabels: map[string]string{}}
	for i := range MaxSelectorRequirements {
		many.MatchLabels[fmt.Sprintf("k%d", i)] = ""
	}
	if err := many.Validate(); err != nil {
		t.Errorf("%d requirements must be allowed: %v", MaxSelectorRequirements, err)
	}
	many.MatchExpressions = []*LabelRequirement{requirement(keyTier, exists)}

	tooManyValues := make([]string, MaxRequirementValues+1)
	for i := range tooManyValues {
		tooManyValues[i] = fmt.Sprintf("v%d", i)
	}

	invalid := map[string]*LabelSelector{
		"nil":                      nil,
		"empty":                    {},
		"too many requirements":    many,
		"bad key in a pair":        {MatchLabels: map[string]string{keyBad: valProd}},
		"bad value in a pair":      {MatchLabels: map[string]string{keyTier: "pr od"}},
		"bad key in an expression": {MatchExpressions: []*LabelRequirement{requirement("-tier", exists)}},
		"no operator":              {MatchExpressions: []*LabelRequirement{requirement(keyTier, LabelOperator_LABEL_OPERATOR_UNSPECIFIED)}},
		"unknown operator":         {MatchExpressions: []*LabelRequirement{requirement(keyTier, LabelOperator(99))}},
		"in without values":        {MatchExpressions: []*LabelRequirement{requirement(keyTier, in)}},
		"in with a bad value":      {MatchExpressions: []*LabelRequirement{requirement(keyTier, in, "pr od")}},
		"in with too many values":  {MatchExpressions: []*LabelRequirement{requirement(keyTier, in, tooManyValues...)}},
		"exists with values":       {MatchExpressions: []*LabelRequirement{requirement(keyTier, exists, valProd)}},
		"a nil expression":         {MatchExpressions: []*LabelRequirement{nil}},
	}
	for name, s := range invalid {
		err := s.Validate()
		if err == nil {
			t.Errorf("%s: must be invalid", name)
			continue
		}
		if !errors.Is(err, ErrInvalidSelector) {
			t.Errorf("%s: error %v must wrap ErrInvalidSelector", name, err)
		}
	}
	// A syntax problem inside a selector is still a label problem.
	if err := (&LabelSelector{MatchLabels: map[string]string{keyBad: ""}}).Validate(); !errors.Is(err, ErrInvalidLabel) {
		t.Errorf("a bad key must also wrap ErrInvalidLabel: %v", err)
	}
}

// What every invalid selector has in common: it matches nothing, so a
// service that forgets to validate still never selects by accident.
func TestInvalidSelectorsThatCannotBeEvaluatedMatchNothing(t *testing.T) {
	labels := map[string]string{keyTier: valProd}
	for _, s := range []*LabelSelector{
		nil,
		{},
		{MatchExpressions: []*LabelRequirement{requirement(keyTier, LabelOperator_LABEL_OPERATOR_UNSPECIFIED)}},
		{MatchExpressions: []*LabelRequirement{nil}},
		{MatchExpressions: []*LabelRequirement{requirement(keyTier, LabelOperator_LABEL_OPERATOR_IN)}},
	} {
		if s.Matches(labels) {
			t.Errorf("selector %v must match nothing", s)
		}
	}
}
