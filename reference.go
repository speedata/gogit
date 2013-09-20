// Copyright (c) 2013 Patrick Gundlach, speedata (Berlin, Germany)
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package gogit

import (
	"bytes"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
)

type Reference struct {
	Name       string
	Oid        *Oid
	dest       string
	repository *Repository
}

func (ref *Reference) resolveInfo() (*Reference, error) {
	destRef := new(Reference)
	destRef.Name = ref.dest

	destpath := filepath.Join(ref.repository.Rootdir, "info", "refs")
	_, err := os.Stat(destpath)
	if err != nil {
		return nil, err
	}
	infoContents, err := ioutil.ReadFile(destpath)
	if err != nil {
		return nil, err
	}

	r := regexp.MustCompile("([[:xdigit:]]+)\t(.*)\n")
	refs := r.FindAllStringSubmatch(string(infoContents), -1)
	for _, v := range refs {
		if v[2] == ref.dest {
			oid, err := NewOidFromString(v[1])
			if err != nil {
				return nil, err
			}
			destRef.Oid = oid
			return destRef, nil
		}
	}

	return nil, errors.New("Could not resolve info/refs")
}

// Resolve a reference (e.g. "HEAD") to a real object
func (r *Reference) Resolve() (*Reference, error) {
	destRef := new(Reference)
	destRef.Name = r.dest

	destpath := filepath.Join(r.repository.Rootdir, r.dest)
	_, err := os.Stat(destpath)
	if os.IsNotExist(err) {
		// dest does not exist, let's parse info/refs, which comes from git gc(??)
		return r.resolveInfo()
	} else if err != nil {
		return nil, err
	}
	b, err := ioutil.ReadFile(destpath)
	if err != nil {
		return nil, err
	}

	oid, err := NewOidFromString(string(bytes.TrimSpace(b)))
	if err != nil {
		return nil, err
	}
	destRef.Oid = oid
	return destRef, nil
}

// For compatibility with git2go. Return Oid from referece (same as getting .Oid directly)
func (r *Reference) Target() *Oid {
	return r.Oid
}
