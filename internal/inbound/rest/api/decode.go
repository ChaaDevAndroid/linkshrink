package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golinkshrink.com/linkshrink/internal/inbound/rest/httperr"
)

const maxBodyBytes = 1 << 20 // 1 MB

// decodeJSON reads one JSON object from the request body into dst.
// On failure it has ALREADY written the error response and returns false:
// the caller must just return.

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	// MaxBytesReader stops reading after the limit and makes the server close the
	// connection, so a client can't stream gigabytes at us.
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	dec := json.NewDecoder(r.Body)
	// gap 1: a field not present in the target struct becomes an error instead of being ignored
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		writeDecodeError(w, err)
		return false
	}

	// gap 2: Decode stops after the FIRST value. If the body is valid, the next read must
	// hit the end of the body (io.EOF). Anything else is trailing data after the object.
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		// The second read can also hit the size limit, so check that before blaming the client.
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			writeError(w, http.StatusRequestEntityTooLarge, httperr.CodeBodyTooLarge, "request body too large")
			return false
		}
		writeError(w, http.StatusBadRequest, httperr.CodeInvalidJSON, "body must contain a single JSON object")
		return false
	}
	return true
}

// writeDecodeError turns a json.Decoder error into a precise client-facing message (gap 3).
// The order of the checks matters: we go from most specific to most generic.
func writeDecodeError(w http.ResponseWriter, err error) {
	// Size first: a body cut by MaxBytesReader can also look like a syntax error.
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		writeError(w, http.StatusRequestEntityTooLarge, httperr.CodeBodyTooLarge, "request body too large")
		return
	}

	// io.EOF with nothing read means the body was empty.
	if errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, httperr.CodeInvalidJSON, "request body must not be empty")
		return
	}

	// The body stopped in the middle of a value, like {"url":
	// Careful: this is NOT a *json.SyntaxError, it is the separate io.ErrUnexpectedEOF.
	if errors.Is(err, io.ErrUnexpectedEOF) {
		writeError(w, http.StatusBadRequest, httperr.CodeInvalidJSON, "body is not valid JSON (it ends too early)")
		return
	}

	// Real syntax error. Offset is the byte position where the parser gave up.
	if se, ok := errors.AsType[*json.SyntaxError](err); ok {
		writeError(w, http.StatusBadRequest, httperr.CodeInvalidJSON,
			fmt.Sprintf("body is not valid JSON (at position %d)", se.Offset))
		return
	}

	// Valid JSON but the wrong type, like {"url":123}. Field is the JSON field name.
	if te, ok := errors.AsType[*json.UnmarshalTypeError](err); ok {
		if te.Field == "" { // the top-level value itself has the wrong type, like []
			writeError(w, http.StatusBadRequest, httperr.CodeInvalidJSON, "body must be a JSON object")
			return
		}
		writeError(w, http.StatusBadRequest, httperr.CodeInvalidJSON,
			fmt.Sprintf("field %q has the wrong type, expected %s", te.Field, te.Type))
		return
	}

	// DisallowUnknownFields has no typed error: the only way to detect it is the message prefix.
	// The message looks like: json: unknown field "urll"  (the name is already quoted)
	const unknownPrefix = "json: unknown field "
	if strings.HasPrefix(err.Error(), unknownPrefix) {
		writeError(w, http.StatusBadRequest, httperr.CodeInvalidJSON,
			"unknown field "+strings.TrimPrefix(err.Error(), unknownPrefix))
		return
	}

	// Anything we didn't foresee: generic message, never the raw Go error (it could leak internals).
	writeError(w, http.StatusBadRequest, httperr.CodeInvalidJSON, "request body must be valid JSON")
}
