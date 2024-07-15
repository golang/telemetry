// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package content

import (
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/telemetry/internal/testenv"
)

func TestServer_ServeHTTP(t *testing.T) {
	testenv.NeedsGo1Point(t, 23) // output of some http helpers changed in Go 1.23
	fsys := os.DirFS("testdata")
	server := Server(fsys,
		Handler("/data", handleTemplate(fsys)),
		Handler("/json", handleJSON()),
		Handler("/text", handleText()),
		Handler("/teapot", handleTeapot()),
		Handler("/error", handleError()),
	)

	tests := []struct {
		path     string
		wantOut  string
		wantCode int
	}{
		{
			"/index.html",
			"redirect.html.out",
			http.StatusMovedPermanently,
		},
		{
			"/index",
			"redirect.out",
			http.StatusMovedPermanently,
		},
		{
			"/json",
			"json.out",
			http.StatusOK,
		},
		{
			"/text",
			"text.out",
			http.StatusOK,
		},
		{
			"/error",
			"error.out",
			http.StatusBadRequest,
		},
		{
			"/teapot",
			"teapot.out",
			http.StatusTeapot,
		},
		{
			"/style.css",
			"style.css.out",
			http.StatusOK,
		},
		{
			"/",
			"index.html.out",
			http.StatusOK,
		},
		{
			"/data",
			"data.html.out",
			http.StatusOK,
		},
		{
			"/markdown",
			"markdown.md.out",
			http.StatusOK,
		},
		{
			"/404",
			"404.html.out",
			http.StatusNotFound,
		},
		{
			"/subdir",
			"subdir/index.html.out",
			http.StatusOK,
		},
		{
			"/noindex/",
			"noindex/noindex.html.out",
			http.StatusOK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req, err := http.NewRequest("GET", tt.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			server.ServeHTTP(rr, req)
			got := strings.TrimSpace(rr.Body.String())
			data, err := os.ReadFile(path.Join("testdata", tt.wantOut))
			if err != nil {
				t.Fatal(err)
			}
			wantBody := strings.TrimSpace(string(data))
			if diff := cmp.Diff(wantBody, got); diff != "" {
				t.Errorf("GET %s response body mismatch (-want, +got):\n%s", tt.path, diff)
			}
			if rr.Code != tt.wantCode {
				t.Errorf("GET %s response code = %d, want %d", tt.path, rr.Code, tt.wantCode)
			}
		})
	}
}

func Test_stat(t *testing.T) {
	fsys := os.DirFS("testdata")
	tests := []struct {
		urlPath string
		want    string
	}{
		{"/", "index.html"},
		{"/markdown", "markdown.md"},
		{"/sub/path", "sub/path"},
	}
	for _, tt := range tests {
		t.Run(tt.urlPath, func(t *testing.T) {
			if got, _ := stat(fsys, tt.urlPath); got != tt.want {
				t.Errorf("stat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func handleTemplate(fsys fs.FS) HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) error {
		return Template(w, fsys, "data.html", nil, "Data from Handler", http.StatusOK)
	}
}

func handleJSON() HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) error {
		return JSON(w, struct{ Data string }{Data: "Data"}, http.StatusOK)
	}
}

func handleText() HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) error {
		return Text(w, "Hello, World!", http.StatusOK)
	}
}

func handleTeapot() HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		return Status(w, http.StatusTeapot)
	}
}

func handleError() HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		return Error(errors.New("Bad Request"), http.StatusBadRequest)
	}
}
