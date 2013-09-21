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
	"log"
)

type Commit struct {
	Author    *Signature
	Committer *Signature
	treeId    *Oid
	message   string
	parents   []string // sha1 strings
}

// // Return the commit message
// func (ci *Commit) Message() string {
// 	return ci.message
// }

// // Return parent number n (0-based index)
// func (ci *Commit) Parent(n int) *Commit {
// }

// // Return oid of the parent number n (0-based index)
// func (ci *Commit) ParentId(n int) *Oid {
// }

// Return the (root) tree of this commit
func (ci *Commit) Tree() (*Tree, error) {
	t := new(Tree)
	log.Println("New tree")
	return t, nil
}

// Return oid of the (root) tree of this commit
func (ci *Commit) TreeId() *Oid {
	return ci.treeId
}

// // Return the number of parents of the commit. 0 if this is the
// // root commit, otherwise 1,2,...
// func (ci *Commit) ParentCount() int {
// }

func parseCommitData(data []byte) (*Commit, error) {
	commit := new(Commit)
	commit.parents = make([]string, 0, 1)
	log.Println(string(data))
	// we now have the contents of the commit object. Let's investigate...
	nextline := 0
l:
	for {
		eol := bytes.IndexByte(data[nextline:], '\n')
		switch {
		case eol > 0:
			line := data[nextline : nextline+eol]
			spacepos := bytes.IndexByte(line, ' ')
			reftype := line[:spacepos]
			switch string(reftype) {
			case "tree":
				oid, err := NewOidFromString(string(line[spacepos+1:]))
				if err != nil {
					return nil, err
				}
				commit.treeId = oid
			case "parent":
				// A commit can have one or more parents
				commit.parents = append(commit.parents, string(line[spacepos+1:]))
			case "author":
				sig, err := newSignatureFromCommitline(line[spacepos+1:])
				if err != nil {
					return nil, err
				}
				commit.Author = sig
			case "committer":
				sig, err := newSignatureFromCommitline(line[spacepos+1:])
				if err != nil {
					return nil, err
				}
				commit.Committer = sig
			}
			nextline = nextline + eol + 1
		case eol == 0:
			commit.message = string(data[nextline+1:])
			nextline = nextline + 1
		default:
			break l
		}
	}
	return commit, nil
}

// Find the commit object in the repository.
func (repos *Repository) LookupCommit(oid *Oid) (*Commit, error) {
	data, err := repos.getRawObject(oid)
	if err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return parseCommitData(data)
}
