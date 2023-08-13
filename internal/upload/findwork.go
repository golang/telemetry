// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package upload

import (
	"os"
	"path/filepath"
	"strings"
)

// files to handle
type work struct {
	// absolute file names
	countfiles []string // count files to process
	readyfiles []string // old reports to upload
	// relative names
	uploaded map[string]bool // reports that have been uploaded
}

// find all the files that look like counter files or reports
// that need to be uploaded. (There may be unexpected leftover files
// and uploading is supposed to be idempotent.)
func findWork(localdir, uploaddir string) work {
	var ans work
	fis, err := os.ReadDir(localdir)
	if err != nil {
		logger.Printf("could not read %s, progress impossible (%v)", localdir, err)
		return ans
	}
	// count files end in .v1.count
	// reports end in .json. If they are not to be uploaded they
	// start with local.
	for _, fi := range fis {
		if strings.HasSuffix(fi.Name(), ".v1.count") {
			fname := filepath.Join(localdir, fi.Name())
			if stillOpen(fname) {
				continue
			}
			ans.countfiles = append(ans.countfiles, fname)
		} else if strings.HasPrefix(fi.Name(), "local.") {
			// skip
		} else if strings.HasSuffix(fi.Name(), ".json") {
			ans.readyfiles = append(ans.readyfiles, filepath.Join(localdir, fi.Name()))
		}
	}
	fis, err = os.ReadDir(uploaddir)
	if err != nil {
		os.MkdirAll(uploaddir, 0777)
		return ans
	}
	// There should be only one of these per day; maybe sometime
	// we'll want to clean the directory.
	ans.uploaded = make(map[string]bool)
	for _, fi := range fis {
		if strings.HasSuffix(fi.Name(), ".json") {
			ans.uploaded[fi.Name()] = true
		}
	}
	return ans
}
