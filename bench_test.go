package gogit

import "testing"

var (
	sinkblob *Blob
	sinkref  *Reference
	sinkrepo *Repository
)

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

func BenchmarkOpenRepository(b *testing.B) {
	var err error
	for i := 0; i < b.N; i++ {
		sinkrepo, err = OpenRepository("_testdata/testrepo.git")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLookupBlob(b *testing.B) {
	repos, err := OpenRepository("_testdata/testrepo.git")
	oid := mustOidFromString(b, "6c493ff740f9380390d5c9ddef4af18697ac9375")
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sinkblob, err = repos.LookupBlob(oid)
		if err != nil {
			b.Fatal(err)
		}
	}
}
