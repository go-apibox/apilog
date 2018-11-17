package apilog

import (
	"bytes"
	"mime"
	"net/http"
)

type ResponseRecorder struct {
	rw http.ResponseWriter

	Code int
	Body *bytes.Buffer
}

func NewRecorder(rw http.ResponseWriter) *ResponseRecorder {
	return &ResponseRecorder{
		rw:   rw,
		Code: 200,
		Body: new(bytes.Buffer),
	}
}

func (recorder *ResponseRecorder) Header() http.Header {
	return recorder.rw.Header()
}

func (recorder *ResponseRecorder) Write(buf []byte) (int, error) {
	contentType := recorder.Header().Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	// 只记录API响应
	if err == nil {
		if mediaType == "application/json" || mediaType == "application/javascript" {
			recorder.Body.Write(buf)
		}
	}

	return recorder.rw.Write(buf)
}

func (recorder *ResponseRecorder) WriteHeader(code int) {
	recorder.Code = code
	recorder.rw.WriteHeader(code)
}
