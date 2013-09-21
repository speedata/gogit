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
)

// A tree entry is similar to a directory entry (file name, type) in a real file system.
type TreeEntry struct {
	Filemode int
	Name     string
	Id       *Oid
	Type     ObjectType
}

// Who am I?
type ObjectType int

const (
	OBJ_COMMIT ObjectType = iota
	OBJ_SYMLINK
	OBJ_TREE
	OBJ_BLOB
)

func (t ObjectType) String() string {
	switch t {
	case OBJ_COMMIT:
		return "Commit"
	case OBJ_TREE:
		return "Tree"
	case OBJ_BLOB:
		return "Blob"
	case OBJ_SYMLINK:
		return "Symlink"
	default:
		return ""
	}
}

// A tree is a flat directory listing.
type Tree struct {
	TreeEntries []*TreeEntry
}

// Parse tree information from the (uncompressed) raw
// data from the tree object.
func parseTreeData(data []byte) (*Tree, error) {
	tree := new(Tree)
	tree.TreeEntries = make([]*TreeEntry, 0, 10)
	l := len(data)
	pos := 0
	for pos < l {
		te := new(TreeEntry)
		spacepos := bytes.IndexByte(data[pos:], ' ')
		switch string(data[pos : pos+spacepos]) {
		case "100644":
			te.Filemode = 0100644
			te.Type = OBJ_BLOB
		case "120000":
			te.Filemode = 0120000
			te.Type = OBJ_SYMLINK
		case "160000":
			te.Filemode = 0160000
			te.Type = OBJ_COMMIT
		case "40000":
			te.Filemode = 0040000
			te.Type = OBJ_TREE
		default:
			return nil, errors.New("unknown type: " + string(data[pos:pos+2]))
		}
		pos += spacepos + 1
		zero := bytes.IndexByte(data[pos:], 0)
		te.Name = string(data[pos : pos+zero])
		pos += zero + 1
		oid, err := NewOid(data[pos : pos+20])
		if err != nil {
			return nil, err
		}
		te.Id = oid
		pos = pos + 20
		tree.TreeEntries = append(tree.TreeEntries, te)
	}
	return tree, nil
}

// Find the entry in this directory (tree) with the given name.
func (t *Tree) EntryByName(name string) *TreeEntry {
	for _, v := range t.TreeEntries {
		if v.Name == name {
			return v
		}
	}
	return nil
}

// Get the n-th entry of this tree (0 = first entry). You can also access
// t.TreeEntries[index] directly.
func (t *Tree) EntryByIndex(index int) *TreeEntry {
	if index >= len(t.TreeEntries) {
		return nil
	}
	return t.TreeEntries[index]
}

// Get the number of entries in the directory (tree). Same as
// len(t.TreeEntries).
func (t *Tree) EntryCount() int {
	return len(t.TreeEntries)
}
