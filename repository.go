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
	"compress/zlib"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
)

type Repository struct {
	Rootdir    string
	indexfiles []*index
}

type SHA1 [20]byte

const (
	_PACK_OBJ_COMMIT               = 0x10
	_PACK_OBJ_TREE                 = 0x20
	_PACK_OBJ_BLOB                 = 0x30
	_PACK_OBJ_TAG                  = 0x40
	_PACK_OBJ_DELTA_ENCODED_OFFSET = 0x60
	_PACK_OBJ_DELTA_ENCODED_OBJID  = 0x70
)

// index file
type index struct {
	indexpath    string
	packpath     string
	packversion  uint32
	offsetValues map[SHA1]uint64
}

func readIdxFile(path string) (*index, error) {
	ifile := &index{}
	ifile.indexpath = path
	ifile.packpath = path[0:len(path)-3] + "pack"
	idx, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if !bytes.HasPrefix(idx, []byte{255, 't', 'O', 'c'}) {
		return nil, errors.New("Not version 2 index file")
	}
	pos := 8
	var fanout [256]uint32
	for i := 0; i < 256; i++ {
		fanout[i] = uint32(idx[pos])<<24 + uint32(idx[pos+1])<<16 + uint32(idx[pos+2])<<8 + uint32(idx[pos+3])
		pos = pos + 4
	}
	numObjects := int(fanout[255])
	ids := make([]SHA1, numObjects)

	for i := 0; i < numObjects; i++ {
		for j := 0; j < 20; j++ {
			ids[i][j] = idx[pos+j]
		}
		pos = pos + 20
	}
	// skip crc32
	pos = pos + 4*numObjects

	excessLen := len(idx) - 258*4 - 28*numObjects - 40
	var offsetValues8 []uint64
	if excessLen > 0 {
		// We have an index table, so let's read it first
		offsetValues8 = make([]uint64, excessLen/8)
		for i := 0; i < excessLen/8; i++ {
			offsetValues8[i] = uint64(idx[pos])<<070 + uint64(idx[pos+1])<<060 + uint64(idx[pos+2])<<050 + uint64(idx[pos+3])<<040 + uint64(idx[pos+4])<<030 + uint64(idx[pos+5])<<020 + uint64(idx[pos+6])<<010 + uint64(idx[pos+7])
			pos = pos + 8
		}
	}
	ifile.offsetValues = make(map[SHA1]uint64, numObjects)
	pos = 258*4 + 24*numObjects
	for i := 0; i < numObjects; i++ {
		offset := uint32(idx[pos])<<24 + uint32(idx[pos+1])<<16 + uint32(idx[pos+2])<<8 + uint32(idx[pos+3])
		offset32ndbit := offset & 0x80000000
		offset31bits := offset & 0x7FFFFFFF
		if offset32ndbit == 0x80000000 {
			// it's an index entry
			ifile.offsetValues[ids[i]] = offsetValues8[offset31bits]
		} else {
			ifile.offsetValues[ids[i]] = uint64(offset31bits)
		}
		pos = pos + 4
	}
	// sha1Packfile := idx[pos : pos+20]
	// sha1Index := idx[pos+21 : pos+40]
	fi, err := os.Open(ifile.packpath)
	if err != nil {
		return nil, err
	}
	defer fi.Close()

	packVersion := make([]byte, 8)
	_, err = fi.Read(packVersion)
	if err != nil {
		return nil, err
	}
	if !bytes.HasPrefix(packVersion, []byte{'P', 'A', 'C', 'K'}) {
		return nil, errors.New("Pack file does not start with 'PACK'")
	}
	ifile.packversion = uint32(packVersion[4])<<24 + uint32(packVersion[5])<<16 + uint32(packVersion[6])<<8 + uint32(packVersion[7])
	return ifile, nil
}

// If the object is stored in its own file (i.e not in a pack file),
// this function returns the full path to the object file.
// It does not test if the file exists.
func filepathFromSHA1(rootdir, sha1 string) string {
	return filepath.Join(rootdir, "objects", sha1[:2], sha1[2:])
}

// Read deflated object from the file.
func readFromZip(file *os.File, start int64, inflatedSize int64) ([]byte, error) {
	_, err := file.Seek(start, os.SEEK_SET)
	if err != nil {
		return nil, err
	}

	rc, err := zlib.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	zbuf := make([]byte, inflatedSize)
	_, err = rc.Read(zbuf)
	if err != nil {
		return nil, err
	}
	return zbuf, nil
}

// buf must be large enough to read the number
func readLittleEndianBase128Number(buf []byte) (int64, int) {
	zpos := 0
	toread := int64(buf[zpos] & 0x7f)
	shift := uint64(0)
	for buf[zpos]&0x80 > 0 {
		zpos += 1
		shift += 7
		toread |= int64(buf[zpos]&0x7f) << shift
	}
	zpos += 1
	return toread, zpos
}

// We take “delta instructions”, a base object, the expected length
// of the resulting object and we can create a resulting object.
func applyDelta(b []byte, base []byte, resultLen int64) []byte {
	resultObject := make([]byte, resultLen)
	var resultpos uint64
	var basepos uint64
	zpos := 0
	for zpos < len(b) {
		// two modes: copy and insert. copy reads offset and len from the delta
		// instructions and copy len bytes from offset into the resulting object
		// insert takes up to 127 bytes and insert them into the
		// resulting object
		opcode := b[zpos]
		zpos += 1
		if opcode&0x80 > 0 {
			// Copy from base to dest

			copy_offset := uint64(0)
			copy_length := uint64(0)
			shift := uint(0)
			for i := 0; i < 4; i++ {
				if opcode&0x01 > 0 {
					copy_offset |= uint64(b[zpos]) << shift
					zpos += 1
				}
				opcode >>= 1
				shift += 8
			}

			shift = 0
			for i := 0; i < 3; i++ {
				if opcode&0x01 > 0 {
					copy_length |= uint64(b[zpos]) << shift
					zpos += 1
				}
				opcode >>= 1
				shift += 8
			}
			if copy_length == 0 {
				copy_length = 1 << 16
			}
			basepos = copy_offset
			for i := uint64(0); i < copy_length; i++ {
				resultObject[resultpos] = base[basepos]
				resultpos++
				basepos++
			}
		} else if opcode > 0 {
			// insert n bytes at the end of the resulting object. n==opcode
			for i := 0; i < int(opcode); i++ {
				resultObject[resultpos] = b[zpos]
				resultpos++
				zpos++
			}
		} else {
			log.Fatal("opcode == 0")
		}
	}
	// TODO: check if resultlen == resultpos
	return resultObject
}

// Read from a pack file (given by path) at position offset. If this is a
// non-delta object, the bytes are just returned, if the object
// is a deltafied-object, we have to apply the delta to base objects
// before hand.
func readObjectBytes(path string, offset uint64) ([]byte, error) {
	offsetInt := int64(offset)
	file, err := os.Open(path)
	defer file.Close()
	if err != nil {
		return nil, err
	}
	pos, err := file.Seek(offsetInt, os.SEEK_SET)
	if err != nil {
		return nil, err
	}
	if pos != offsetInt {
		return nil, errors.New("Seek went wrong")
	}
	buf := make([]byte, 1024)
	n, err := file.Read(buf)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, errors.New("Nothing read from pack file")
	}
	pos = int64(0)
	objecttype := buf[pos] & 0x70

	shift := []byte{0, 4, 11} // some more, probably 18, 25, 32
	uncompressedLength := int64(buf[pos] & 0x0F)
	for buf[pos]&0x80 > 0 {
		pos = pos + 1
		uncompressedLength = uncompressedLength + (int64(buf[pos]&0x7F) << shift[pos])
	}
	pos = pos + 1

	if int64(len(buf))-pos < uncompressedLength {
		// todo for later: implement when the requested file length
		// is larger than the buffer we've just read
		return nil, errors.New("not implemented yet - read more")
	}

	var baseObjectOffset uint64
	switch objecttype {
	case _PACK_OBJ_COMMIT, _PACK_OBJ_TREE, _PACK_OBJ_BLOB:
		// commit
		b, err := readFromZip(file, offsetInt+pos, uncompressedLength)
		return b, err
	case _PACK_OBJ_TAG:
		log.Fatal("not implemented yet")
	case _PACK_OBJ_DELTA_ENCODED_OFFSET:
		// DELTA_ENCODED object w/ offset to base
		// Read the offset first, then calculate the starting point
		// of the base object
		num := int64(buf[pos]) & 0x7f
		for buf[pos]&0x80 > 0 {
			pos = pos + 1
			num = ((num + 1) << 7) | int64(buf[pos]&0x7f)
		}
		baseObjectOffset = uint64(offsetInt - num)
		pos = pos + 1
	case _PACK_OBJ_DELTA_ENCODED_OBJID:
		// DELTA_ENCODED object w/ base BINARY_OBJID
		log.Fatal("not implemented yet")
	}

	base, err := readObjectBytes(path, baseObjectOffset)
	if err != nil {
		return nil, err
	}
	b, err := readFromZip(file, offsetInt+pos, uncompressedLength)
	if err != nil {
		return nil, err
	}
	zpos := 0
	// This is the length of the base object. Do we need to know it?
	_, bytesRead := readLittleEndianBase128Number(b)
	zpos += bytesRead
	resultObjectLength, bytesRead := readLittleEndianBase128Number(b[zpos:])
	zpos += bytesRead
	resultObject := applyDelta(b[zpos:], base, resultObjectLength)
	return resultObject, nil
}

// Return length as integer from zero terminated string
// and the beginning of the real object
func getLengthZeroTerminated(b []byte) (int, int) {
	i := 0
	var pos int
	for b[i] != 0 {
		i++
	}
	pos = i
	i--
	length := 0
	pow := 1
	for i >= 0 {
		length = length + (int(b[i])-48)*pow
		pow = pow * 10
		i--
	}
	return length, pos + 1
}

// Read the contents of the file at path
// Return the content type, the contents of the file and error, if any
func readFile(path string) (string, []byte, error) {
	file, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	r, err := zlib.NewReader(file)
	if err != nil {
		return "", nil, err
	}
	defer r.Close()
	first_buffer_size := 1024
	b := make([]byte, first_buffer_size)
	n, err := r.Read(b)
	if err != nil {
		return "", nil, err
	}
	spaceposition := bytes.IndexByte(b, ' ')

	// "tree", "commit", "blob", ...
	objecttype := string(b[:spaceposition])

	// length starts at the position after the space
	length, objstart := getLengthZeroTerminated(b[spaceposition+1:])

	objstart = objstart + spaceposition + 1
	// if the size of our buffer is less than the object length + the bytes
	// in front of the object (example: "commit 234\0") we need to increase
	// the size of the buffer and read the rest. Warning: this should only
	// be done on small files
	if n < length+objstart {
		remaining_bytes := make([]byte, length-first_buffer_size+objstart)
		n, err = r.Read(remaining_bytes)
		if n != length-first_buffer_size+objstart {
			return "", nil, errors.New("Remaining bytes do not match")
		}
		if err != nil {
			return "", nil, err
		}
		b = append(b, remaining_bytes...)
	}
	return objecttype, b[objstart : objstart+length], nil
}

func (repos *Repository) getRawObject(oid *Oid) ([]byte, error) {
	// first we need to find out where the commit is stored
	objpath := filepathFromSHA1(repos.Rootdir, oid.String())
	_, err := os.Stat(objpath)
	if os.IsNotExist(err) {
		// doesn't exist, let's look if we find the object somewhere else
		for _, indexfile := range repos.indexfiles {
			if offset := indexfile.offsetValues[oid.Bytes]; offset != 0 {
				return readObjectBytes(indexfile.packpath, offset)
			}
		}
		return nil, errors.New("Object not found")
	}
	_, data, err := readFile(objpath)
	return data, err
}

// Open the repository at the given path.
func OpenRepository(path string) (*Repository, error) {
	root := new(Repository)
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	root.Rootdir = path
	fm, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !fm.IsDir() {
		return nil, errors.New(fmt.Sprintf("%q is not a directory."))
	}

	indexfiles, err := filepath.Glob(filepath.Join(path, "objects/pack/*idx"))
	if err != nil {
		return nil, err
	}
	root.indexfiles = make([]*index, len(indexfiles))
	for i, indexfile := range indexfiles {
		// packfile := indexfile[0:len(indexfile)-len("idx")] + ".pack"
		idx, err := readIdxFile(indexfile)
		if err != nil {
			return nil, err
		}
		root.indexfiles[i] = idx
	}

	return root, nil
}

// A typical Git repository consists of objects (path objects/ in the root directory)
// and of references to HEAD, branches, tags and such. We can't read the HEAD directly,
// we first need to resolve the reference to a real object.
func (repos *Repository) LookupReference(name string) (*Reference, error) {
	ref := new(Reference)
	ref.repository = repos
	f, err := ioutil.ReadFile(filepath.Join(repos.Rootdir, name))
	if err != nil {
		return nil, err
	}
	ref.Name = name
	rexp := regexp.MustCompile("ref: (.*)\n")
	allMatches := rexp.FindAllStringSubmatch(string(f), 1)
	if len(allMatches) < 1 && len(allMatches[0]) < 1 {
		return nil, errors.New("Could not parse reference, no match for regexp 'ref: (.*)\\n'.")
	}
	ref.dest = allMatches[0][1]
	return ref, nil
}

// Return the root directory of the repository. Same as reading from Rootdir.
// For compatibility with git2go.
func (repos *Repository) Path() string {
	return repos.Rootdir
}
