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

type TreeEntry struct {
	Filemode int
	Name     string
	Id       *Oid
	// Type
}

type Tree struct {
	treeentries []*TreeEntry
}

// Parse tree information from the (uncompressed) raw
// data from the tree object.
func parseTreeData(data []byte) (*Tree, error) {
	tree := new(Tree)
	tree.treeentries = make([]*TreeEntry, 0, 10)
	l := len(data)
	pos := 0
	for pos < l {
		te := new(TreeEntry)
		spacepos := bytes.IndexByte(data[pos:], ' ')
		switch string(data[pos : pos+spacepos]) {
		case "100644":
			te.Filemode = 0100644
		case "120000":
			te.Filemode = 0120000
		case "160000":
			te.Filemode = 0160000
		case "40000":
			te.Filemode = 0040000
		default:
			log.Println("unknown type:", string(data[pos:pos+2]))
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
		tree.treeentries = append(tree.treeentries, te)
	}
	return tree, nil
}

// Find the entry in this directory (tree) with the given name.
func (t *Tree) EntryByName(name string) *TreeEntry {
	for _, v := range t.treeentries {
		if v.Name == name {
			return v
		}
	}
	return nil
}

// Get the n-th entry of this tree (0 = first entry).
func (t *Tree) EntryByIndex(index int) *TreeEntry {
	if index >= len(t.treeentries) {
		return nil
	}
	return t.treeentries[index]
}

// Get the number of entries in the directory (tree).
func (t *Tree) EntryCount() int {
	return len(t.treeentries)
}
