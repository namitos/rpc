package rpc

import (
	"encoding/json"
	"io"
	"net/http"
)

func setDefaultHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
}

func SendAPIError(w http.ResponseWriter, err error) {
	setDefaultHeaders(w)
	errTxt := err.Error()
	output, _ := json.Marshal(Output{
		Error: &OutputError{Message: errTxt},
	})
	status := http.StatusInternalServerError
	if errTxt == "not implemented" {
		status = http.StatusNotImplemented
	}
	if errTxt == "forbidden" {
		status = http.StatusForbidden
	}
	w.WriteHeader(status)
	w.Write(output)
}

func SendAPIResult(w http.ResponseWriter, out any, err error) {
	if err != nil {
		SendAPIError(w, err)
		return
	}
	outJSON, err := json.Marshal(out)
	if err != nil {
		SendAPIError(w, err)
		return
	}
	setDefaultHeaders(w)
	w.Write(outJSON)
}

func WrapRPC(resultFn func(io.ReadCloser) (any, error)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		out, err := resultFn(r.Body)
		SendAPIResult(w, out, err)
	}
}

func SetCORSHeaders(allowOrigins []string, w http.ResponseWriter, r *http.Request) bool {
	setDefaultHeaders(w)
	headers := w.Header()
	allowOrigin := ""
	origin := r.Header.Get("Origin")
	if len(allowOrigins) == 0 {
		allowOrigin = "*"
	} else {
		for _, o := range allowOrigins {
			if o == origin {
				allowOrigin = o
				break
			}
		}
	}
	if allowOrigin != "" {
		headers.Set("Access-Control-Allow-Origin", allowOrigin)
	}
	if r.Method == "OPTIONS" {
		headers.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		headers.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		return true
	}
	return false
}
