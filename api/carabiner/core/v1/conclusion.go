// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package core

// Outcome maps a Conclusion down to the cdevents-standard three-value Outcome.
// Conclusion retains detail (cancelled, skipped, …) the Outcome cannot express;
// this is the canonical reduction used wherever an Outcome is required (e.g. on
// the wire). Conclusions that could not complete normally (cancelled, timed out,
// stale, action required) reduce to ERROR; results that did not fail (skipped,
// neutral) reduce to SUCCESS.
func (c Conclusion) Outcome() Outcome {
	switch c {
	case Conclusion_CONCLUSION_SUCCESS:
		return Outcome_OUTCOME_SUCCESS
	case Conclusion_CONCLUSION_FAILURE:
		return Outcome_OUTCOME_FAILURE
	case Conclusion_CONCLUSION_CANCELLED,
		Conclusion_CONCLUSION_TIMED_OUT,
		Conclusion_CONCLUSION_STALE,
		Conclusion_CONCLUSION_ACTION_REQUIRED:
		return Outcome_OUTCOME_ERROR
	case Conclusion_CONCLUSION_SKIPPED,
		Conclusion_CONCLUSION_NEUTRAL:
		return Outcome_OUTCOME_SUCCESS
	case Conclusion_CONCLUSION_UNSPECIFIED:
		return Outcome_OUTCOME_UNSPECIFIED
	default:
		return Outcome_OUTCOME_UNSPECIFIED
	}
}
