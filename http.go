package revel

import (
	"bytes"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/BSP-Mosaic/teltech-glog"
)

type Request struct {
	*http.Request
	ContentType     string
	Format          string // "html", "xml", "json", or "txt"
	AcceptLanguages AcceptLanguages
	Locale          string
}

type Response struct {
	Status      int
	ContentType string

	Out http.ResponseWriter
}

func NewResponse(w http.ResponseWriter) *Response {
	return &Response{Out: w}
}

func NewRequest(r *http.Request) *Request {
	return &Request{
		Request:         r,
		ContentType:     ResolveContentType(r),
		Format:          ResolveFormat(r),
		AcceptLanguages: ResolveAcceptLanguage(r),
	}
}

// Write the header (for now, just the status code).
// The status may be set directly by the application (c.Response.Status = 501).
// if it isn't, then fall back to the provided status code.
func (resp *Response) WriteHeader(defaultStatusCode int, defaultContentType string) {
	if resp.Status == 0 {
		resp.Status = defaultStatusCode
	}
	if resp.ContentType == "" {
		resp.ContentType = defaultContentType
	}
	resp.Out.Header().Set("Content-Type", resp.ContentType)
	resp.Out.WriteHeader(resp.Status)
}

// Get the content type.
// e.g. From "multipart/form-data; boundary=--" to "multipart/form-data"
// If none is specified, returns "text/html" by default.
func ResolveContentType(req *http.Request) string {
	contentType := req.Header.Get("Content-Type")
	if contentType == "" {
		return "text/html"
	}
	return strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
}

// Resolve the accept request header.
func ResolveFormat(req *http.Request) string {
	accept := req.Header.Get("accept")

	switch {
	case accept == "",
		strings.HasPrefix(accept, "*/*"),
		strings.Contains(accept, "application/xhtml"),
		strings.Contains(accept, "text/html"):
		return "html"
	case strings.Contains(accept, "application/json"),
		strings.Contains(accept, "text/javascript"):
		return "json"
	case strings.Contains(accept, "application/xml"),
		strings.Contains(accept, "text/xml"):
		return "xml"
	case strings.Contains(accept, "text/plain"):
		return "txt"
	}

	return "html"
}

// FormatToContentType returns an appropriate default content type for the
// response, given the request format.
func FormatToContentType(format string) string {
	switch format {
	case "html":
		return "text/html; charset=utf-8"
	case "txt":
		return "text/plain; charset=utf-8"
	case "xml":
		return "text/xml; charset=utf-8"
	case "json":
		return "application/json; charset=utf-8"
	}
	panic("unrecognized format: " + format)
}

// A single language from the Accept-Language HTTP header.
type AcceptLanguage struct {
	Language string
	Quality  float32
}

// A collection of sortable AcceptLanguage instances.
type AcceptLanguages []AcceptLanguage

func (al AcceptLanguages) Len() int           { return len(al) }
func (al AcceptLanguages) Swap(i, j int)      { al[i], al[j] = al[j], al[i] }
func (al AcceptLanguages) Less(i, j int) bool { return al[i].Quality > al[j].Quality }
func (al AcceptLanguages) String() string {
	output := bytes.NewBufferString("")
	for i, language := range al {
		output.WriteString(fmt.Sprintf("%s (%1.1f)", language.Language, language.Quality))
		if i != len(al)-1 {
			output.WriteString(", ")
		}
	}
	return output.String()
}

// Resolve the Accept-Language header value.
//
// The results are sorted using the quality defined in the header for each language range with the
// most qualified language range as the first element in the slice.
//
// See the HTTP header fields specification
// (http://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.4) for more details.
func ResolveAcceptLanguage(req *http.Request) AcceptLanguages {
	header := req.Header.Get("Accept-Language")
	if header == "" {
		return nil
	}

	acceptLanguageHeaderValues := strings.Split(header, ",")
	acceptLanguages := make(AcceptLanguages, len(acceptLanguageHeaderValues))

	for i, languageRange := range acceptLanguageHeaderValues {
		if qualifiedRange := strings.Split(languageRange, ";q="); len(qualifiedRange) == 2 {
			quality, error := strconv.ParseFloat(qualifiedRange[1], 32)
			if error != nil {
				glog.Warningf("Detected malformed Accept-Language header quality in '%s', assuming quality is 1", languageRange)
				acceptLanguages[i] = AcceptLanguage{qualifiedRange[0], 1}
			} else {
				acceptLanguages[i] = AcceptLanguage{qualifiedRange[0], float32(quality)}
			}
		} else {
			acceptLanguages[i] = AcceptLanguage{languageRange, 1}
		}
	}

	sort.Sort(acceptLanguages)
	return acceptLanguages
}
