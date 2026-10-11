package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// gota make sure it doesnt pass 1mb, hell even 1mb is too big. need to implement limits to real uploads
const maxJSONBody = 1 << 20

// ErrorBody is the ONLY error shape the api will ever return. the frontend writes one error parser against this and never neds another
type ErrorBody struct {
	Code    string            `json:"code"`             // stable and machine-readable, example: "email_taken"
	Message string            `json:"message"`          // human-readable, can change at will
	Fields  map[string]string `json:"fields,omitempty"` // map
}

// with this a client can check if body.error without knowing the status code, and success bodies can neevr be mistaken for errors.
type errorEnvelope struct {
	Error ErrorBody `json:"error"`
}

// so this maps a form field to what's wrong with it so the frontend is able to show messages under the right input. (registeration needs it)
type ValidationErrors map[string]string

// add keeps the first error per field, and validation checks the basics first (stuff like "required", before checking "invalid format")
func (v ValidationErrors) Add(field, msg string) {
	if _, exists := v[field]; !exists {
		v[field] = msg
	}
}

// this makes validation used as a go error, as long as its an actual error given the len(v)
func (v ValidationErrors) Any() bool { return len(v) > 0 }

// lets ValidationErrors double as a go error so a service can return it and the caller can read the fields back out later
func (v ValidationErrors) Error() string { return "validation failed" }

func WriteJSON(w http.ResponseWriter, status int, v any) {
	// headers gotta be set before writeheader
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}

	// error is ignored on purpose, the status line is already sent so that's that, it only fails when
	// the client disconnects or v cant be encoded, which is a programming bug that'll be caught.
	_ = json.NewEncoder(w).Encode(v)
}

func WriteError(w http.ResponseWriter, status int, code, msg string) {
	WriteJSON(w, status, errorEnvelope{Error: ErrorBody{Code: code, Message: msg}})
}

//! this used to be the old impl of the validation, but instead, it can simply return the error
//! in a struct containing all the issues, rather than hardcoding certain values
// func WRiteValidation(w http.ResponseWriter, fields ValidationErrors) {
// 	WriteJSON(w, http.StatusBadRequest, errorEnvelope{Error: ErrorBody{
// 		Code:    "validation_failed",
// 		Message: "some fields are invalid",
// 		Fields:  fields,
// 	}})
// }

// a WriteError and per-field message, for stuff belonging to a form input
func WriteErrorFields(w http.ResponseWriter, status int, code, msg string, fields ValidationErrors) {
	WriteJSON(w, status, errorEnvelope{Error: ErrorBody{Code: code, Message: msg, Fields: fields}})
}

// modularised because why not. also, it used to be one function that checks for the err, but it used to be more limiting
func WriteValidation(w http.ResponseWriter, fields ValidationErrors) {
	WriteErrorFields(w, http.StatusBadRequest, "validation_failed", "some fields are invalid", fields)
}

// DecodeJSON decodes the req body into dst (a dst is a destination pointer). Every error it returns has a message that's safe to show
// to the client, so handles can writeerror 400 bad req

func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	// wrap body before reading
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	dec := json.NewDecoder(r.Body)
	// reject fields the handler does not expect. typos like first_name = 5400 instead of an empty field. ALso blocks sneaking in fields like "is_admin"
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {

		// just turn go's error types into readable messages
		var syntaxErr *json.SyntaxError
		var typeErr *json.UnmarshalTypeError
		var maxErr *http.MaxBytesError

		//! cant believe im using switch in the big 26
		// this makes sure it goes through every case of an error, before checking the exact error
		// we've done this check in a lot of places since just checking if there is an error is not enough
		// we have to now the exact kind of error
		switch {
		case errors.Is(err, io.EOF):
			return errors.New("Error: request body must not be empty")
		case errors.As(err, &syntaxErr), errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("Error: request body is not a valid JSON")
		case errors.As(err, &typeErr):
			return fmt.Errorf("Error: fiield %q has the wrong type", typeErr.Field)
		case strings.HasPrefix(err.Error(), "json: unknown field"):
			return fmt.Errorf("Error: unknown field %s", strings.TrimPrefix(err.Error(), "json: unknown field"))
		case errors.As(err, &maxErr):
			return fmt.Errorf("Error: request body must not exceed %d bytes", maxErr.Limit)
		default:
			return err
		}
	}
	//* DisallowUnknownFields(), used with JSON decoder, to enforce strict parsing
	//* it returns an error in case the JSON input contains keys that dont match the any exported fields
	//! simply check for broken or incomplete inputs in the json that gets returned from the frontend
	// without DisallowUnknownFields, an unknown field is excluded, but the request still succeeds
	// however, with it, an unknown field makes Decode return an error. the whole thing fails with 400
	// check for trailing data; &struct{}{} is just a throwaway destination for the second value
	//? note: with unmarshal, lets say there is a field in the struct that the form doesnt ask for it,
	//? you can still exploit that by sending it, since it does schematically exist: mass assignment vulnerability in the api
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("Error: request body must contain a single JSON object")
	}
	return nil
}
