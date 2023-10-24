// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package content implements a basic web serving framework.
//
// # Content Server
//
// A content server is an http.Handler that serves requests from a file system.
// Use Server(fsys) to create a new content server.
//
// The server is defined primarily by the content of its file system fsys,
// which holds files to be served. It renders markdown files and golang
// templates into HTML.
//
// # Page Rendering
//
// A request for a path like "/page" will search the file system for
// "page.md", "page.html", "page/index.md", and "page/index.html" and
// render HTML output for the first file found.
//
// Partial templates with the extension ".tmpl" at the root of the file system
// and in the same directory as the requested page are included in the
// html/template execution step to allow for sharing and composing logic from
// multiple templates.
//
// Markdown templates must have an html layout template set in the frontmatter
// section. The markdown content is available to the layout template as the
// field `{{.Content}}`.
package content

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// contentServer serves requests for a given file system and renders html
// templates.
type contentServer struct {
	fsys     fs.FS
	fserv    http.Handler
	handlers map[string]HandlerFunc
}

type HandlerFunc func(http.ResponseWriter, *http.Request) error

func (f HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := f(w, r); err != nil {
		handleErr(w, r, err, http.StatusInternalServerError)
	}
}

// Server returns a handler that serves HTTP requests with the contents
// of the file system rooted at fsys. For requests to a path without an
// extension, the server will search fsys for markdown or html templates
// first by appending .md, .html, /index.md, and /index.html to the
// requested url path.
//
// The default behavior of looking for templates within fsys can be overriden
// by using an optional set of content handlers.
//
// For example, a server can be constructed for a file system with a single
// template, “index.html“, in a directory, “content“, and a handler:
//
//	  fsys := os.DirFS("content")
//		 s := content.Server(fsys,
//			 content.Handler("/", func(w http.ReponseWriter, _ *http.Request) error {
//			 	 return content.Template(w, fsys, "index.html", nil, http.StatusOK)
//			 }))
//
// or without a handler:
//
//	content.Server(os.DirFS("content"))
//
// Both examples will render the template index.html for requests to "/".
func Server(fsys fs.FS, handlers ...*handler) http.Handler {
	fserv := http.FileServer(http.FS(fsys))
	hs := make(map[string]HandlerFunc)
	for _, h := range handlers {
		if _, ok := hs[h.path]; ok {
			panic("multiple registrations for " + h.path)
		}
		hs[h.path] = h.fn
	}
	return &contentServer{fsys, fserv, hs}
}

type handler struct {
	path string
	fn   HandlerFunc
}

func Handler(path string, h HandlerFunc) *handler {
	return &handler{path, h}
}

func (c *contentServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if len(r.URL.Path) > 255 {
		handleErr(w, r, errors.New("url too long"), http.StatusBadRequest)
		return
	}

	if h, ok := c.handlers[r.URL.Path]; ok {
		h.ServeHTTP(w, r)
		return
	}

	ext := path.Ext(r.URL.Path)
	if ext == ".md" || ext == ".html" {
		http.Redirect(w, r, strings.TrimSuffix(r.URL.Path, ext), http.StatusMovedPermanently)
		return
	}

	filepath, err := stat(c.fsys, r.URL.Path)
	if errors.Is(err, fs.ErrNotExist) {
		handleErr(w, r, errors.New(http.StatusText(http.StatusNotFound)), http.StatusNotFound)
		return
	}
	if err == nil {
		if strings.HasSuffix(r.URL.Path, "/index") {
			http.Redirect(w, r, strings.TrimSuffix(r.URL.Path, "/index"), http.StatusMovedPermanently)
			return
		}
		switch path.Ext(filepath) {
		case ".html":
			err = Template(w, c.fsys, filepath, nil, http.StatusOK)
		case ".md":
			err = markdown(w, c.fsys, filepath, http.StatusOK)
		default:
			c.fserv.ServeHTTP(w, r)
		}
	}
	if err != nil {
		handleErr(w, r, err, http.StatusInternalServerError)
	}
}

// Template executes a template response.
func Template(w http.ResponseWriter, fsys fs.FS, tmplPath string, data any, code int) error {
	patterns, err := tmplPatterns(fsys, tmplPath)
	if err != nil {
		return err
	}
	patterns = append(patterns, tmplPath)
	tmpl, err := template.ParseFS(fsys, patterns...)
	if err != nil {
		return err
	}
	name := path.Base(tmplPath)
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return err
	}
	if code != 0 {
		w.WriteHeader(code)
	}
	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	if _, err := w.Write(buf.Bytes()); err != nil {
		return err
	}
	return nil
}

// JSON encodes data as JSON response with a status code.
func JSON(w http.ResponseWriter, data any, code int) error {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(data); err != nil {
		return err
	}
	if code != 0 {
		w.WriteHeader(code)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	if _, err := w.Write(buf.Bytes()); err != nil {
		return err
	}
	return nil
}

// Text formats data as a text response with a status code.
func Text(w http.ResponseWriter, data any, code int) error {
	var buf bytes.Buffer
	if _, err := fmt.Fprint(&buf, data); err != nil {
		return err
	}
	if code != 0 {
		w.WriteHeader(code)
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	if _, err := w.Write(buf.Bytes()); err != nil {
		return err
	}
	return nil
}

// Text renders an http status code as a text response.
func Status(w http.ResponseWriter, code int) error {
	if code < http.StatusBadRequest {
		return Text(w, http.StatusText(code), code)
	}
	return Error(errors.New(http.StatusText(code)), code)
}

// Error annotates an error with http status information.
func Error(err error, code int) error {
	return &contentError{err, code}
}

type contentError struct {
	err  error
	Code int
}

func (e *contentError) Error() string { return e.err.Error() }

// handleErr writes an error as an HTTP response with a status code.
func handleErr(w http.ResponseWriter, req *http.Request, err error, code int) {
	if cerr, ok := err.(*contentError); ok {
		code = cerr.Code
	}
	if code == http.StatusInternalServerError {
		http.Error(w, http.StatusText(http.StatusInternalServerError), code)
	} else {
		http.Error(w, err.Error(), code)
	}
}

// markdown renders a markdown template as html.
func markdown(w http.ResponseWriter, fsys fs.FS, tmplPath string, code int) error {
	markdown, err := fs.ReadFile(fsys, tmplPath)
	if err != nil {
		return err
	}
	md := goldmark.New(
		goldmark.WithParserOptions(
			parser.WithHeadingAttribute(),
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
			html.WithXHTML(),
		),
		goldmark.WithExtensions(
			extension.GFM,
			extension.NewTypographer(),
			meta.Meta,
		),
	)
	var content bytes.Buffer
	ctx := parser.NewContext()
	if err := md.Convert(markdown, &content, parser.WithContext(ctx)); err != nil {
		return err
	}
	data := meta.Get(ctx)
	if data == nil {
		data = map[string]interface{}{}
	}
	data["Content"] = template.HTML(content.String())
	layout, ok := data["Layout"]
	if !ok {
		return errors.New("missing layout for template " + tmplPath)
	}
	return Template(w, fsys, layout.(string), data, code)
}

// stat trys to coerce a urlPath into an openable file then returns the
// file path.
func stat(fsys fs.FS, urlPath string) (string, error) {
	cleanPath := path.Clean(strings.TrimPrefix(urlPath, "/"))
	ext := path.Ext(cleanPath)
	filePaths := []string{cleanPath}
	if ext == "" || ext == "." {
		md := cleanPath + ".md"
		html := cleanPath + ".html"
		indexMD := path.Join(cleanPath, "index.md")
		indexHTML := path.Join(cleanPath, "index.html")
		filePaths = []string{md, html, indexMD, indexHTML, cleanPath}
	}
	var p string
	var err error
	for _, p = range filePaths {
		if _, err = fs.Stat(fsys, p); err == nil {
			break
		}
	}
	return p, err
}

// tmplPatters generates a slice of file patterns to use in template.ParseFS.
func tmplPatterns(fsys fs.FS, tmplPath string) ([]string, error) {
	var patterns []string
	globs := []string{"*.tmpl", path.Join(path.Dir(tmplPath), "*.tmpl")}
	for _, g := range globs {
		matches, err := fs.Glob(fsys, g)
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, matches...)
	}
	return patterns, nil
}
