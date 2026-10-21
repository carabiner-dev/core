// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"fmt"
	"path"
	"strings"
)

// A codebase is a buildable unit inside a repository, as unpack finds them:
// a language and the directory it lives in. unpack names one
// "<language>:<path>"; the functions here fix the canonical form of that
// identifier so unpack, keystone and everything in between produce the same
// string for the same codebase, which is what keys the platform's record of
// it within its repository.

const (
	// CodebaseRootPath is the canonical path of a codebase at the root of
	// its repository.
	CodebaseRootPath = "."

	// CodebaseLanguageLabelKey is the label the platform sets on every
	// codebase with its language, under the reserved prefix, so a selector
	// reaches "the Go code" without anyone labelling it. A codebase's path
	// is not a label: label values cannot hold a slash.
	CodebaseLanguageLabelKey = ReservedLabelDomain + "/language"

	maxCodebaseLanguageLength = maxLabelNameLength
)

// ErrInvalidCodebaseID marks a codebase identifier, language or path that
// cannot be made canonical.
var ErrInvalidCodebaseID = errors.New("invalid codebase identifier")

// ValidateCodebaseLanguage checks a language as unpack names one: 1 to 63
// characters of [a-z0-9_.-], starting and ending with a letter or digit,
// which is also what a label value allows, so the language label is always
// valid.
func ValidateCodebaseLanguage(language string) error {
	if language == "" || len(language) > maxCodebaseLanguageLength ||
		!isLowerAlphanumeric(language[0]) || !isLowerAlphanumeric(language[len(language)-1]) {
		return fmt.Errorf("%w: language %q must be 1 to %d characters of [a-z0-9_.-], starting and ending with a letter or digit", ErrInvalidCodebaseID, language, maxCodebaseLanguageLength)
	}
	for i := range len(language) {
		if c := language[i]; !isLowerAlphanumeric(c) && c != '_' && c != '.' && c != '-' {
			return fmt.Errorf("%w: language %q must be 1 to %d characters of [a-z0-9_.-], starting and ending with a letter or digit", ErrInvalidCodebaseID, language, maxCodebaseLanguageLength)
		}
	}
	return nil
}

// CanonicalCodebasePath returns the canonical form of a codebase's path
// within its repository: forward slashes, no leading "./", no trailing
// slash, no empty or "." segments, and "." for the repository root. A path
// that is absolute, uses backslashes or climbs out of the repository is
// refused.
func CanonicalCodebasePath(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("%w: empty path", ErrInvalidCodebaseID)
	}
	if strings.ContainsRune(p, '\\') {
		return "", fmt.Errorf("%w: path %q uses backslashes", ErrInvalidCodebaseID, p)
	}
	if strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("%w: path %q is absolute; it must be relative to the repository root", ErrInvalidCodebaseID, p)
	}
	clean := path.Clean(p)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("%w: path %q leaves the repository", ErrInvalidCodebaseID, p)
	}
	return clean, nil
}

// CodebaseSourceID returns the canonical identifier of a codebase,
// "<language>:<canonical path>", as unpack's GenerateCodebaseID does once the
// path is canonical.
func CodebaseSourceID(language, p string) (string, error) {
	if err := ValidateCodebaseLanguage(language); err != nil {
		return "", err
	}
	canonical, err := CanonicalCodebasePath(p)
	if err != nil {
		return "", err
	}
	return language + ":" + canonical, nil
}

// ParseCodebaseSourceID splits an identifier into its language and canonical
// path, accepting the forms unpack writes ("golang:./backend") and the
// canonical one ("golang:backend").
func ParseCodebaseSourceID(id string) (language, canonicalPath string, err error) {
	language, p, ok := strings.Cut(id, ":")
	if !ok {
		return "", "", fmt.Errorf("%w: %q has no colon between language and path", ErrInvalidCodebaseID, id)
	}
	if err := ValidateCodebaseLanguage(language); err != nil {
		return "", "", err
	}
	canonicalPath, err = CanonicalCodebasePath(p)
	if err != nil {
		return "", "", err
	}
	return language, canonicalPath, nil
}
