gogit
=====

Pure Go read access to a git repository.

State: in development, not yet usable

License: free software (MIT License)

Installation: just run "go get github.com/speedata/gogit"

API documentation: http://godoc.org/github.com/speedata/gogit

Example
-------

Sample application to list the latest directory (recursively):

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
        head, err := ref.Resolve()
        if err != nil {
            log.Fatal(err)
        }
        ci, err := repository.LookupCommit(head.Oid)
        if err != nil {
            log.Fatal(err)
        }
        tree, err := ci.Tree()
        if err != nil {
            log.Fatal(err)
        }
        tree.Walk(walk)
    }
