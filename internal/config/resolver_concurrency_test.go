package config

import (
	"sync"
	"testing"
)

func TestResolverConcurrentRepoLoading(t *testing.T) {
	cfg := Default()
	resolver := NewResolver(&cfg)
	repos := []string{t.TempDir(), t.TempDir(), t.TempDir()}
	var wg sync.WaitGroup
	for i := range 24 {
		wg.Go(func() {
			repo := repos[i%len(repos)]
			first, err := resolver.ConfigForRepo(repo)
			if err != nil {
				t.Error(err)
				return
			}
			second, err := resolver.ConfigForRepo(repo)
			if err != nil {
				t.Error(err)
				return
			}
			if first != second {
				t.Error("concurrent callers did not share cached config")
			}
		})
	}
	wg.Wait()
}
