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

// A codebase URI names a codebase inside the platform's registry, with an
// optional revision, so the same codebase is spelled the same way wherever
// it appears: on an attestation subject (without the revision, which the
// subject's digest carries), inside an SBOM as a package name (with it), in
// a policy or a selector.
//
//	codebase://<system>/<slug>[@<revision>]#<language>:<path>
//
// The authority and path are the repository as keystone names it: the
// system by its registry name ("github", not "github.com") and the slug
// under it, which may hold as many segments as the system needs (a GitLab
// subgroup). The revision, when present, is the source revision the
// codebase is taken at (a commit sha). The fragment is the codebase's
// canonical identifier, which is why it comes last: it holds a colon and
// slashes of its own, and everything before the last "#" is the repository.

// CodebaseURIScheme is the scheme of a codebase URI.
const CodebaseURIScheme = "codebase"

// ErrInvalidCodebaseURI marks a codebase URI that does not parse.
var ErrInvalidCodebaseURI = errors.New("invalid codebase URI")

// CodebaseURI is a parsed codebase URI.
type CodebaseURI struct {
	// System is the registry's name for the source system ("github").
	System string
	// Slug is the repository's slug under the system ("acme/widgets";
	// "platform/tools/widgets" for a GitLab subgroup).
	Slug string
	// Revision is the source revision, empty when the URI names the codebase
	// at no revision in particular.
	Revision string
	// SourceID is the codebase's canonical identifier ("golang:backend").
	SourceID string
}

// String renders the URI. Revision is left out when empty.
func (u CodebaseURI) String() string {
	var b strings.Builder
	b.WriteString(CodebaseURIScheme)
	b.WriteString("://")
	b.WriteString(u.System)
	b.WriteByte('/')
	b.WriteString(u.Slug)
	if u.Revision != "" {
		b.WriteByte('@')
		b.WriteString(u.Revision)
	}
	b.WriteByte('#')
	b.WriteString(u.SourceID)
	return b.String()
}

// WithoutRevision returns the URI naming the codebase at no revision.
func (u CodebaseURI) WithoutRevision() CodebaseURI {
	u.Revision = ""
	return u
}

// Validate checks the parts: a system name, a slug of non-empty segments,
// a revision without a slash or an at sign, and a canonical source id.
func (u CodebaseURI) Validate() error {
	if u.System == "" || !isLowerAlphanumeric(u.System[0]) {
		return fmt.Errorf("%w: system %q must be the registry's lowercase name", ErrInvalidCodebaseURI, u.System)
	}
	for i := range len(u.System) {
		if c := u.System[i]; !isLowerAlphanumeric(c) && c != '-' && c != '.' {
			return fmt.Errorf("%w: system %q must be the registry's lowercase name", ErrInvalidCodebaseURI, u.System)
		}
	}
	if u.Slug == "" {
		return fmt.Errorf("%w: no repository slug", ErrInvalidCodebaseURI)
	}
	for segment := range strings.SplitSeq(u.Slug, "/") {
		if segment == "" || strings.ContainsAny(segment, "@#") {
			return fmt.Errorf("%w: slug %q has an empty segment or a reserved character", ErrInvalidCodebaseURI, u.Slug)
		}
	}
	if strings.ContainsAny(u.Revision, "/@#") {
		return fmt.Errorf("%w: revision %q holds a reserved character", ErrInvalidCodebaseURI, u.Revision)
	}
	language, p, err := ParseCodebaseSourceID(u.SourceID)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidCodebaseURI, err)
	}
	if canonical := language + ":" + p; canonical != u.SourceID {
		return fmt.Errorf("%w: source id %q is not canonical (%q)", ErrInvalidCodebaseURI, u.SourceID, canonical)
	}
	return nil
}

// NewCodebaseURI builds and validates a URI from a repository's system and
// slug, a source id in any spelling core canonicalizes, and an optional
// revision.
func NewCodebaseURI(system, slug, revision, sourceID string) (CodebaseURI, error) {
	language, p, err := ParseCodebaseSourceID(sourceID)
	if err != nil {
		return CodebaseURI{}, fmt.Errorf("%w: %w", ErrInvalidCodebaseURI, err)
	}
	u := CodebaseURI{System: system, Slug: slug, Revision: revision, SourceID: language + ":" + p}
	if err := u.Validate(); err != nil {
		return CodebaseURI{}, err
	}
	return u, nil
}

// ParseCodebaseURI parses a codebase URI. The fragment is split at the last
// "#" and the revision at the last "@" before it, so slugs may hold slashes.
func ParseCodebaseURI(s string) (CodebaseURI, error) {
	rest, ok := strings.CutPrefix(s, CodebaseURIScheme+"://")
	if !ok {
		return CodebaseURI{}, fmt.Errorf("%w: %q does not start with %s://", ErrInvalidCodebaseURI, s, CodebaseURIScheme)
	}
	hash := strings.LastIndex(rest, "#")
	if hash < 0 {
		return CodebaseURI{}, fmt.Errorf("%w: %q has no #<language>:<path> fragment", ErrInvalidCodebaseURI, s)
	}
	repository, sourceID := rest[:hash], rest[hash+1:]
	var revision string
	if at := strings.LastIndex(repository, "@"); at >= 0 {
		repository, revision = repository[:at], repository[at+1:]
		if revision == "" {
			return CodebaseURI{}, fmt.Errorf("%w: %q has an empty revision", ErrInvalidCodebaseURI, s)
		}
	}
	system, slug, found := strings.Cut(repository, "/")
	if !found {
		return CodebaseURI{}, fmt.Errorf("%w: %q has no repository slug after the system", ErrInvalidCodebaseURI, s)
	}
	return NewCodebaseURI(system, slug, revision, sourceID)
}
