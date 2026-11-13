package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// defaultJSONBodyLimit caps JSON request bodies for handlers without a larger need.
const defaultJSONBodyLimit int64 = 1 << 20

// errBodyTooLarge means decodeJSON has already answered with 413; the caller
// must return without writing another response.
var errBodyTooLarge = errors.New("request body too large")

// decodeJSON decodes one JSON value from the request body into dst, reading at
// most limit bytes. An oversized body is answered with 413 and returns
// errBodyTooLarge; any other decode error is returned for the caller to report.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any, limit int64) error {
	return decodeJSONWith(w, r, limit, func(d *json.Decoder) error { return d.Decode(dst) })
}

// decodeJSONWith is decodeJSON for callers that configure the decoder or read
// more than one value, such as strict field or single-document checks.
func decodeJSONWith(w http.ResponseWriter, r *http.Request, limit int64, decode func(*json.Decoder) error) error {
	err := decode(json.NewDecoder(http.MaxBytesReader(w, r.Body, limit)))
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("request body exceeds the %d byte limit", limit))
		return errBodyTooLarge
	}
	return err
}
