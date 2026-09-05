package pages

import (
	"bytes"
	"context"
	"sync"
	"testing"

	"tabletopper/internal/queries"
	"tabletopper/internal/session"

	"github.com/a-h/templ"
)

// Every page used to assign into a package-level settings struct on each
// render, which the race detector flags the moment two renders overlap. The
// layout now takes its title and body per call; this keeps it that way.
func TestPagesRenderConcurrently(t *testing.T) {
	pages := map[string]func() error{
		"homepage":      func() error { return render(Homepage(session.UserSession{})) },
		"characters":    func() error { return render(Characters([]queries.Character{})) },
		"new-character": func() error { return render(NewCharacter()) },
		"assets":        func() error { return render(MapAssets([]queries.Asset{})) },
		"sign-in":       func() error { return render(SignIn()) },
		"tos":           func() error { return render(TOS()) },
	}

	var wg sync.WaitGroup
	for name, page := range pages {
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := page(); err != nil {
					t.Errorf("%s: %v", name, err)
				}
			}()
		}
	}
	wg.Wait()
}

func render(c templ.Component) error {
	var buf bytes.Buffer
	return c.Render(context.Background(), &buf)
}
