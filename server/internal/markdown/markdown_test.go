package markdown_test

import (
	"strings"
	"testing"

	"tabletopper/internal/markdown"
)

// keepAll is the ImageSource for the tests that are not about images: every
// destination survives unchanged, so anything missing from the output was
// removed by something else.
func keepAll(dest string) (string, bool) { return dest, true }

func render(t *testing.T, body string, images markdown.ImageSource) string {
	t.Helper()

	out, err := markdown.Render(body, images)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	return out
}

// The defence the whole read view rests on. A body is whatever was in the
// textarea, so a writer can put a <script> in one; a shared entry renders for
// strangers, and self-XSS becomes everyone's XSS the moment it does.
func TestRawHTMLInABodyIsNotRendered(t *testing.T) {
	bodies := map[string]string{
		"script block":  "<script>alert(1)</script>",
		"inline markup": "Hello <img src=x onerror=alert(1)> world",
		"iframe":        "<iframe src=\"https://example.com\"></iframe>",
	}

	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			out := render(t, body, keepAll)

			for _, forbidden := range []string{"<script", "<iframe", "onerror"} {
				if strings.Contains(out, forbidden) {
					t.Errorf("raw HTML reached the output: found %q in\n%s", forbidden, out)
				}
			}
		})
	}
}

func TestDangerousLinkDestinationsAreDropped(t *testing.T) {
	for _, scheme := range []string{"javascript:alert(1)", "vbscript:msgbox(1)", "data:text/html,<script>alert(1)</script>"} {
		t.Run(scheme, func(t *testing.T) {
			out := render(t, "[click me]("+scheme+")", keepAll)

			if strings.Contains(out, "href=\""+scheme) {
				t.Errorf("a %s destination survived\n%s", scheme, out)
			}
			// The text is still there -- goldmark drops the destination and
			// keeps the anchor, which is the right amount of damage.
			if !strings.Contains(out, "click me") {
				t.Errorf("the link text was lost as well\n%s", out)
			}
		})
	}
}

func TestAnOrdinaryLinkSurvives(t *testing.T) {
	out := render(t, "[the wiki](https://example.com/orcs)", keepAll)

	if !strings.Contains(out, `href="https://example.com/orcs"`) {
		t.Errorf("an http link should render as one\n%s", out)
	}
}

// The editor's own marks, and the two that need saying: strikethrough is an
// extension CommonMark does not carry, and a hard break is serialised by
// prosemirror-markdown as a trailing backslash rather than two spaces.
func TestTheEditorsOwnMarksRender(t *testing.T) {
	cases := map[string]struct{ body, want string }{
		"bold":          {"**loud**", "<strong>loud</strong>"},
		"italic":        {"*quiet*", "<em>quiet</em>"},
		"strikethrough": {"~~gone~~", "<del>gone</del>"},
		"code":          {"`spell`", "<code>spell</code>"},
		"blockquote":    {"> said the dragon", "<blockquote>"},
		"heading":       {"## The road", "<h2>The road</h2>"},
		"bullets":       {"- one\n- two", "<ul>"},
		"numbers":       {"1. one\n2. two", "<ol>"},
		"hard break":    {"one\\\ntwo", "<br>"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			out := render(t, tc.body, keepAll)

			if !strings.Contains(out, tc.want) {
				t.Errorf("wanted %q in the output\n%s", tc.want, out)
			}
		})
	}
}

// The rewrite half of ImageSource: a share serves an entry's pictures from its
// own URLs, and the body carries the owner's.
func TestAnImageIsRenderedAtTheURLTheSourceReturns(t *testing.T) {
	out := render(t, "![a map](/characters/C/journal/E/images/A)", func(dest string) (string, bool) {
		if dest != "/characters/C/journal/E/images/A" {
			t.Errorf("the source was asked about %q", dest)
		}

		return "/share/tok/images/A", true
	})

	if !strings.Contains(out, `src="/share/tok/images/A"`) {
		t.Errorf("the destination was not rewritten\n%s", out)
	}
	if !strings.Contains(out, `alt="a map"`) {
		t.Errorf("the alt text was lost\n%s", out)
	}
}

// The strip half, and the reason the CSP on the entry page exists: a foreign
// URL in a body is a request every reader of a shared page would make to
// somebody else's server.
func TestAForeignImageIsRemovedRatherThanRendered(t *testing.T) {
	out := render(t, "![](https://tracker.example/pixel.gif)", func(string) (string, bool) {
		return "", false
	})

	if strings.Contains(out, "<img") {
		t.Errorf("a foreign image was rendered\n%s", out)
	}
	if strings.Contains(out, "tracker.example") {
		t.Errorf("the foreign URL reached the output\n%s", out)
	}
}

// A picture on a line of its own is a paragraph holding one image, so removing
// it would otherwise leave an empty <p> carrying a real paragraph's margin.
func TestRemovingAnImageTakesTheParagraphItWasAlone(t *testing.T) {
	out := render(t, "before\n\n![](https://tracker.example/pixel.gif)\n\nafter", func(string) (string, bool) {
		return "", false
	})

	if strings.Contains(out, "<p></p>") {
		t.Errorf("an empty paragraph was left behind\n%s", out)
	}
	for _, kept := range []string{"before", "after"} {
		if !strings.Contains(out, kept) {
			t.Errorf("the prose around the image was lost: %q\n%s", kept, out)
		}
	}
}

// The image beside it stays. The two are removed one node at a time, so a
// body mixing its own pictures with a foreign one keeps the ones it owns.
func TestOnlyTheForeignImageIsRemoved(t *testing.T) {
	body := "![mine](/characters/C/journal/E/images/A)\n\n![theirs](https://tracker.example/pixel.gif)"
	out := render(t, body, func(dest string) (string, bool) {
		return "/share/tok/images/A", strings.HasPrefix(dest, "/characters/")
	})

	if strings.Count(out, "<img") != 1 {
		t.Errorf("expected exactly one image to survive\n%s", out)
	}
	if !strings.Contains(out, "/share/tok/images/A") {
		t.Errorf("the entry's own image was dropped\n%s", out)
	}
}

func TestAnEmptyBodyRendersNothing(t *testing.T) {
	if out := render(t, "", keepAll); out != "" {
		t.Errorf("an empty entry rendered %q", out)
	}
}
