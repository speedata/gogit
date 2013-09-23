gogit
=====

Pure Go read access to a git repository.

**State**: In development (testing), actively maintained<br>
**Maturity level**: 2 (0-5)<br>
**License**: Free software (MIT License)<br>
**Installation**: Just run `go get github.com/speedata/gogit`<br>
**API documentation**: http://godoc.org/github.com/speedata/gogit<br>
**Contact**: <gundlach@speedata.de>, [@speedata](https://twitter.com/speedata)<br>
**Repository**: https://github.com/speedata/gogit<br>
**Dependencies**: None

Example
-------

Sample application to list the latest directory (recursively):

```Go
package main

import (
    "github.com/speedata/gogit"
    "log"
    "os"
    "path"
    "path/filepath"
)

func walk(dirname string, te *gogit.TreeEntry) int {
    log.Println(path.Join(dirname, te.Name))
    return 0
}

func main() {
    wd, err := os.Getwd()
    if err != nil {
        log.Fatal(err)
    }
    repository, err := gogit.OpenRepository(filepath.Join(wd, "src/github.com/speedata/gogit/_testdata/testrepo.git"))
    if err != nil {
        log.Fatal(err)
    }
    ref, err := repository.LookupReference("HEAD")
    if err != nil {
        log.Fatal(err)
    }
    ci, err := repository.LookupCommit(ref.Oid)
    if err != nil {
        log.Fatal(err)
    }
    ci.tree.Walk(walk)
}
```
