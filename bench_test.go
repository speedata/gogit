package gogit

import "testing"

var sinkref *Reference

func BenchmarkLookupReference(b *testing.B) {
	repos, err := OpenRepository("_testdata/testrepo.git")
	if err != nil {
		b.Fatal(err)
	}

	cases := [...]struct {
		name  string
		ref   string
		errok bool
	}{
		{name: "HEAD", ref: "HEAD"},
		{name: "master", ref: "refs/heads/master"},
		{name: "doesnotexist", ref: "refs/heads/doesnotexist", errok: true},
	}

	for _, bench := range cases {
		b.Run(bench.name, func(b *testing.B) {
			var err error
			for i := 0; i < b.N; i++ {
				sinkref, err = repos.LookupReference(bench.ref)
				if err != nil && !bench.errok {
					b.Fatal(err)
				}
			}
		})
	}
}
