// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package v1

type Object interface {
	Kind() string
}

type Event interface {
	Kind() string
}
