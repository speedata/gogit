package gogit

import (
	"strings"
	"testing"
)

func TestOpen(t *testing.T) {
	_, err := OpenRepository("xxxxxxxx")
	if err == nil {
		t.Fail()
	}
	repos, err := OpenRepository("_testdata/testrepo.git")
	if err != nil {
		t.Error(err)
	}
	ref, err := repos.LookupReference("HEAD")
	if err != nil {
		t.Error(err)
	}
	exp := "HEAD"
	res := ref.Name
	if res != exp {
		t.Error("in ref.Name", res, "is not", exp)
	}
	inforef, err := ref.resolveInfo()
	if err != nil {
		t.Error(err)
	}
	exp = "7647bdef73cde0888222b7ea00f5e83b151a25d0"
	res = inforef.Oid.String()
	if res != exp {
		t.Error("inforef.Oid.String()", res, "is not", exp)
	}
	newref, err := ref.Resolve()
	if err != nil {
		t.Error(err)
	}
	if false {
		_ = newref
	}
	if newref.Oid.String() != "7647bdef73cde0888222b7ea00f5e83b151a25d0" {
		t.Error(newref.Oid.String(), "should be", "7647bdef73cde0888222b7ea00f5e83b151a25d0")
		t.Fail()
	}
}

func TestOid(t *testing.T) {
	oid, err := NewOidFromString("c9cacbcccdcecfd0d1c8c9cacbcccdcecfd0d100")
	if err != nil {
		t.Fail()
	}
	oid2, _ := NewOid([]byte{201, 202, 203, 204, 205, 206, 207, 208, 209, 200, 201, 202, 203, 204, 205, 206, 207, 208, 209, 0})
	if !oid.Equal(oid2) {
		t.Error("oid doesn't match")
	}
}

func TestIdxFile(t *testing.T) {
	idx, err := readIdxFile("_testdata/testrepo.git/objects/pack/pack-efa084d62d89521059a514772fd2966a3a230984.idx")
	if err != nil {
		t.Error("Index file could not be read")
	}
	// A commit:
	// $ git cat-file -p 7647bdef73cde0888222b7ea00f5e83b151a25d0
	// tree b9a560f9a96f89f3a44508689592ef4b10cc5d22
	// parent aebcb66c85f05557b999ced9c60ec275a5cab71d
	// author Patrick Gundlach <gundlach@speedata.de> 1378823654 +0200
	// committer Patrick Gundlach <gundlach@speedata.de> 1378823654 +0200
	//
	// Change symlink to file/add symlink to dir
	oid, _ := NewOidFromString("7647bdef73cde0888222b7ea00f5e83b151a25d0")
	offset := idx.offsetValues[oid.Bytes]
	exp := uint64(12)
	if offset != exp {
		t.Error("Offset should be", exp, "but is", offset)
	}
	b, err := readObjectBytes(idx.packpath, offset)
	if err != nil {
		t.Error(err)
	}
	length := len(b)
	if length != 267 {
		t.Error("Expecting length 267 but got", length)
	}
	prefix := "tree b9a560f9a96f89f3a44508689592ef4b10cc5d22"
	if !strings.HasPrefix(string(b), prefix) {
		t.Error("Expecting", prefix, "got", string(b[:30]))
	}

	// Read a delta-object from a packfile (a tree)
	// $ git cat-file -p e34a238bd4523af233c27b0196c78a7d722e0d0a
	// 040000 tree 1afb926fa71a5e2944c9f726af84dab286303203	dira
	// 040000 tree acb85aafa4bfdaf3af9e709f93ed537dd5214435	dirb
	// 040000 tree 5f47a6026f62a7d26d1c946c66066ec6931920fd	dirc
	// 100644 blob 8287eed4a1022d897d3e2195e5dc40cc71629c48	file1.txt
	// 100644 blob 6c493ff740f9380390d5c9ddef4af18697ac9375	file2.txt
	// 120000 blob 39cd5762dce4e1841f2087c1b896b09c0300ec5a	symlink
	oid, _ = NewOidFromString("e34a238bd4523af233c27b0196c78a7d722e0d0a")
	offset = idx.offsetValues[oid.Bytes]
	exp = uint64(2582)
	if offset != exp {
		t.Error("Offset should be", exp, "but is", offset)
	}
	b, err = readObjectBytes(idx.packpath, offset)
	if err != nil {
		t.Error(err)
	}
	prefix = "40000 dira"
	length = 202
	if !strings.HasPrefix(string(b), prefix) {
		t.Error("Expected prefix", prefix, "but got", string(b[:20]))
	}
	if len(b) != length {
		t.Error("Expecting length 202 but got", len(b))
	}
}

func TestLookupCommit(t *testing.T) {
	repos, err := OpenRepository("_testdata/testrepo.git")
	if err != nil {
		t.Error(err)
	}

	oid, err := NewOidFromString("8496add21eddc0cdc78a121c5df6b41bb685b886")
	if err != nil {
		t.Error(err)
	}

	_, err = repos.LookupCommit(oid)

}

func TestReadLEBase128(t *testing.T) {
	ret, _ := readLittleEndianBase128Number([]byte{0xf7, 0x1})
	if ret != 247 {
		t.Error("Expected 247, got", ret)
	}
	ret, _ = readLittleEndianBase128Number([]byte{0xca, 0x1})
	if ret != 202 {
		t.Error("Expected 202, got", ret)
	}
}

func TestReadCommit(t *testing.T) {
	commitid := "7647bdef73cde0888222b7ea00f5e83b151a25d0"
	commitoid, err := NewOidFromString(commitid)
	if err != nil {
		t.Error(err)
	}
	repos, err := OpenRepository("_testdata/testrepo.git")
	if err != nil {
		t.Error(err)
	}
	commit, err := repos.LookupCommit(commitoid)
	if err != nil {
		t.Error(err)
	}
	treeid := "b9a560f9a96f89f3a44508689592ef4b10cc5d22"
	if commit.TreeId().String() != treeid {
		t.Error("Expected tree", treeid, "but got", commit.TreeId().String())
	}
	if commit.Author.Name != "Patrick Gundlach" {
		t.Error("Expected author name: Patrick Gundlach but got", commit.Author.Name)
	}
	// err is never set
	tree, _ := commit.Tree()
	if ec := tree.EntryCount(); ec != 7 {
		t.Error("Expected 7 entries in the tree, got", ec)
	}
	{
		te := tree.EntryByIndex(2)
		if te.Name != "dirc" {
			t.Error("Wrong entry in tree.EntryByIndex. Expected 'dirc', got", te.Name)
		}
	}
	{
		te := tree.EntryByName("dirc")
		if te.Name != "dirc" {
			t.Error("Wrong entry in tree.EntryByIndex. Expected 'dirc', got", te.Name)
		}
	}
	{
		te := tree.EntryByName("doesnotexist")
		if te != nil {
			t.Error("Expect nil")
		}
	}

}

func BenchmarkSHAtoHex(b *testing.B) {
	sha_bin := []byte{201, 202, 203, 204, 205, 206, 207, 208, 209, 200, 201, 202, 203, 204, 205, 206, 207, 208, 209, 0}
	oid, _ := NewOid(sha_bin)
	for i := 0; i < b.N; i++ {
		oid.String()
	}
}
