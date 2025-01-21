package easyframework

import "net/http"

func Authenticate(ctx *RequestContext, w http.ResponseWriter, r *http.Request) bool {
	//session := r.Header.Get("session")
	return true
}
