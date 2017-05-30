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
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/speedata/mmap-go"
)

// A Repository is the base of all other actions. If you need to lookup a
// commit, tree or blob, you do it from here.
type Repository struct {
	Path       string
	indexfiles []*idxFile
}

type SHA1 [20]byte

// Who am I?
type ObjectType int

const (
	ObjectCommit ObjectType = 0x10
	ObjectTree   ObjectType = 0x20
	ObjectBlob   ObjectType = 0x30
	ObjectTag    ObjectType = 0x40
)

func (t ObjectType) String() string {
	switch t {
	case ObjectCommit:
		return "Commit"
	case ObjectTree:
		return "Tree"
	case ObjectBlob:
		return "Blob"
	default:
		return ""
	}
}

type Object struct {
	Type ObjectType
	Oid  *Oid
}

// idx-file
type idxFile struct {
	packpath string

	fanoutTable [256]int64

	// These tables are sub-slices of the whole idx file as an mmap
	shaTable     []byte
	offsetTable  []byte
	offset8Table []byte
}

func readIdxFile(path string) (*idxFile, error) {
	ifile := &idxFile{}
	ifile.packpath = path[0:len(path)-3] + "pack"

	idxf, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer idxf.Close()
	idxMmap, err := mmap.Map(idxf, mmap.RDONLY, 0)

	if err != nil {
		return nil, err
	}
	if !bytes.HasPrefix(idxMmap, []byte{255, 't', 'O', 'c'}) {
		return nil, errors.New("Not version 2 index file")
	}

	for i := range ifile.fanoutTable {
		pos := 8 + i*4
		u32 := binary.BigEndian.Uint32(idxMmap[pos : pos+4])
		ifile.fanoutTable[i] = int64(u32)
	}
	numObjects := int64(ifile.fanoutTable[byte(255)])

	shaStart := int64(8 + 256*4)
	ifile.shaTable = idxMmap[shaStart : shaStart+20*numObjects]

	offsetStart := shaStart + 24*numObjects
	ifile.offsetTable = idxMmap[offsetStart : offsetStart+4*numObjects]

	offset8Start := offsetStart + 4*numObjects
	ifile.offset8Table = idxMmap[offset8Start : len(idxMmap)-40]

	fi, err := os.Open(ifile.packpath)
	if err != nil {
		return nil, err
	}
	defer fi.Close()

	packVersion := make([]byte, 8)
	_, err = io.ReadFull(fi, packVersion)
	if err != nil {
		return nil, err
	}
	if !bytes.HasPrefix(packVersion, []byte{'P', 'A', 'C', 'K'}) {
		return nil, errors.New("Pack file does not start with 'PACK'")
	}
	return ifile, nil
}

func (idx *idxFile) offsetForSHA(target SHA1) uint64 {
	// Restrict search between shas that start with the correct
	// byte, thanks to the fanoutTable.
	var startSearch int64
	if target[0] > 0 {
		startSearch = idx.fanoutTable[target[0]-1]
	}
	endSearch := idx.fanoutTable[target[0]]

	// Search for the position of the target sha1.
	var exactMatch bool
	found := sort.Search(int(endSearch-startSearch), func(i int) bool {
		cpos := (startSearch + int64(i)) * 20
		comp := bytes.Compare(target[:], idx.shaTable[cpos:cpos+20])
		if comp == 0 {
			exactMatch = true
		}
		return comp <= 0
	})
	if !exactMatch {
		return 0
	}

	// We found it, now read value
	pos := (startSearch + int64(found)) * 4
	offset32 := binary.BigEndian.Uint32(idx.offsetTable[pos : pos+4])
	offset := uint64(offset32)

	// If msb is set, offset is actually an index into the "big" table where each
	// offset is 8 bytes instead of 4 bytes.
	if offset&0x80000000 == 0x80000000 {
		pos := int64(offset&0x7FFFFFFF) * 8
		offset = binary.BigEndian.Uint64(idx.offset8Table[pos : pos+8])
	}

	return offset
}

// If the object is stored in its own file (i.e not in a pack file),
// this function returns the full path to the object file.
// It does not test if the file exists.
func filepathFromSHA1(rootdir, sha1 string) string {
	return filepath.Join(rootdir, "objects", sha1[:2], sha1[2:])
}

// zlibReaderPool holds zlib Readers.
var zlibReaderPool sync.Pool

// Read deflated object from the file.
func readCompressedDataFromFile(file *os.File, start int64, inflatedSize int64) ([]byte, error) {
	_, err := file.Seek(start, os.SEEK_SET)
	if err != nil {
		return nil, err
	}

	z := zlibReaderPool.Get()
	if z != nil {
		err = z.(zlib.Resetter).Reset(file, nil)
	} else {
		z, err = zlib.NewReader(file)
	}
	if z != nil {
		defer zlibReaderPool.Put(z)
	}
	if err != nil {
		return nil, err
	}
	rc := z.(io.ReadCloser)
	defer rc.Close()
	zbuf := make([]byte, inflatedSize)
	if _, err := io.ReadFull(rc, zbuf); err != nil {
		return nil, err
	}
	return zbuf, nil
}

// buf must be large enough to read the number.
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
func applyDelta(b []byte, base []byte, resultLen int64) ([]byte, error) {
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
			return nil, fmt.Errorf("opcode == 0")
		}
	}
	// TODO: check if resultlen == resultpos
	return resultObject, nil
}

// The object length in a packfile is a bit more difficult than
// just reading the bytes. The first byte has the length in its
// lowest four bits, and if bit 7 is set, it means 'more' bytes
// will follow. These are added to the »left side« of the length
func readLenInPackFile(buf []byte) (length int, advance int) {
	advance = 0
	shift := [...]byte{0, 4, 11, 18, 25, 32, 39, 46, 53, 60}
	length = int(buf[advance] & 0x0F)
	for buf[advance]&0x80 > 0 {
		advance += 1
		length += (int(buf[advance]&0x7F) << shift[advance])
	}
	advance++
	return
}

// Read from a pack file (given by path) at position offset. If this is a
// non-delta object, the (inflated) bytes are just returned, if the object
// is a deltafied-object, we have to apply the delta to base objects
// before hand.
func readObjectBytes(path string, offset uint64, sizeonly bool) (ot ObjectType, length int64, data []byte, err error) {
	offsetInt := int64(offset)
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	pos, err := file.Seek(offsetInt, os.SEEK_SET)
	if err != nil {
		return
	}
	if pos != offsetInt {
		err = errors.New("Seek went wrong")
		return
	}
	buf := make([]byte, 1024)
	n, err := file.Read(buf)
	if err != nil {
		if err == io.EOF && n > 0 {
			// Ignore EOF error and move on
			err = nil
		} else {
			return
		}
	}
	if n == 0 {
		err = errors.New("Nothing read from pack file")
		return
	}
	ot = ObjectType(buf[0] & 0x70)

	l, p := readLenInPackFile(buf)
	pos = int64(p)
	length = int64(l)

	var baseObjectOffset uint64
	switch ot {
	case ObjectCommit, ObjectTree, ObjectBlob, ObjectTag:
		if sizeonly {
			// if we are only interested in the size of the object,
			// we don't need to do more expensive stuff
			return
		}

		data, err = readCompressedDataFromFile(file, offsetInt+pos, length)
		return
	case 0x60:
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
	case 0x70:
		// DELTA_ENCODED object w/ base BINARY_OBJID
		err = fmt.Errorf("not implemented yet")
		return
	}
	var base []byte
	ot, _, base, err = readObjectBytes(path, baseObjectOffset, false)
	if err != nil {
		return
	}
	b, err := readCompressedDataFromFile(file, offsetInt+pos, length)
	if err != nil {
		return
	}
	zpos := 0
	// This is the length of the base object. Do we need to know it?
	_, bytesRead := readLittleEndianBase128Number(b)
	zpos += bytesRead
	resultObjectLength, bytesRead := readLittleEndianBase128Number(b[zpos:])
	zpos += bytesRead
	if sizeonly {
		// if we are only interested in the size of the object,
		// we don't need to do more expensive stuff
		length = resultObjectLength
		return
	}

	data, err = applyDelta(b[zpos:], base, resultObjectLength)
	return
}

// Return length as integer from zero terminated string
// and the beginning of the real object
func getLengthZeroTerminated(b []byte) (int64, int64) {
	i := 0
	var pos int
	for b[i] != 0 {
		i++
	}
	pos = i
	i--
	var length int64
	var pow int64
	pow = 1
	for i >= 0 {
		length = length + (int64(b[i])-48)*pow
		pow = pow * 10
		i--
	}
	return length, int64(pos) + 1
}

// Read the contents of the object file at path.
// Return the content type, the contents of the file and error, if any
func readObjectFile(path string, sizeonly bool) (ot ObjectType, length int64, data []byte, err error) {
	var file *os.File
	file, err = os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	r, err := zlib.NewReader(file)
	if err != nil {
		return
	}
	defer r.Close()
	first_buffer_size := int64(1024)
	b := make([]byte, first_buffer_size)
	n, err := r.Read(b)
	if err != nil {
		if err == io.EOF && n > 0 {
			// Ignore EOF error on read
			err = nil
		} else {
			return
		}
	}
	spaceposition := int64(bytes.IndexByte(b, ' '))

	// "tree", "commit", "blob", ...
	objecttypeString := string(b[:spaceposition])

	switch objecttypeString {
	case "blob":
		ot = ObjectBlob
	case "tree":
		ot = ObjectTree
	case "commit":
		ot = ObjectCommit
	case "tag":
		ot = ObjectTag
	}

	// length starts at the position after the space
	var objstart int64
	length, objstart = getLengthZeroTerminated(b[spaceposition+1:])

	if sizeonly {
		// if we are only interested in the size of the object,
		// we don't need to do more expensive stuff
		return
	}

	objstart += spaceposition + 1

	// if the size of our buffer is less than the object length + the bytes
	// in front of the object (example: "commit 234\0") we need to increase
	// the size of the buffer and read the rest. Warning: this should only
	// be done on small files
	if int64(n) < length+objstart {
		remainingSize := length - first_buffer_size + objstart
		remainingBuf := make([]byte, remainingSize)
		n = 0
		var count int64
		for count < remainingSize {
			n, err = r.Read(remainingBuf[count:])
			if err != nil && err != io.EOF {
				return
			}
			count += int64(n)
		}
		b = append(b, remainingBuf...)
	}
	data = b[objstart : objstart+length]
	if err == io.EOF {
		// The last read yielded exactly the right number of bytes
		// as well as an EOF. Ignore the EOF; we succeeded.
		err = nil
	}
	return
}

func (repos *Repository) getRawObject(oid *Oid) (ObjectType, int64, []byte, error) {
	// first we need to find out where the commit is stored
	objpath := filepathFromSHA1(repos.Path, oid.String())
	_, err := os.Stat(objpath)
	if os.IsNotExist(err) {
		// doesn't exist, let's look if we find the object somewhere else
		for _, indexfile := range repos.indexfiles {
			if offset := indexfile.offsetForSHA(oid.Bytes); offset != 0 {
				return readObjectBytes(indexfile.packpath, offset, false)
			}
		}
		return 0, 0, nil, errObjNotFound
	}
	return readObjectFile(objpath, false)
}

// Open the repository at the given path.
func OpenRepository(path string) (*Repository, error) {
	root := new(Repository)
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	root.Path = path
	fm, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !fm.IsDir() {
		return nil, errors.New(fmt.Sprintf("%q is not a directory."))
	}

	indexfiles, err := filepath.Glob(filepath.Join(path, "objects/pack/*.idx"))
	if err != nil {
		return nil, err
	}
	root.indexfiles = make([]*idxFile, len(indexfiles))
	for i, indexfile := range indexfiles {
		idx, err := readIdxFile(indexfile)
		if err != nil {
			return nil, err
		}
		root.indexfiles[i] = idx
	}

	return root, nil
}

// Get the type of an object.
func (repos *Repository) Type(oid *Oid) (ObjectType, error) {
	objtype, _, _, err := repos.getRawObject(oid)
	if err != nil {
		return 0, err
	}
	return objtype, nil
}

// Get (inflated) size of an object.
func (repos *Repository) ObjectSize(oid *Oid) (int64, error) {

	// todo: this is mostly the same as getRawObject -> merge
	// difference is the boolean in readObjectBytes and readObjectFile
	objpath := filepathFromSHA1(repos.Path, oid.String())
	_, err := os.Stat(objpath)
	if os.IsNotExist(err) {
		// doesn't exist, let's look if we find the object somewhere else
		for _, indexfile := range repos.indexfiles {
			if offset := indexfile.offsetForSHA(oid.Bytes); offset != 0 {
				_, length, _, err := readObjectBytes(indexfile.packpath, offset, true)
				return length, err
			}
		}

		return 0, errors.New("Object not found")
	}
	_, length, _, err := readObjectFile(objpath, true)
	return length, err
}
