package body

import "testing"

func TestExtractArticleTextUsesDescendantSelector_whenMatchingClassAppearsOutsideArticle(t *testing.T) {
	// Given a page with the BleepingComputer class both outside and inside the article element.
	markup := `<body><div class="articleBody">Advertisement</div><article><header>Article header</header><div class="articleBody"><p>Security article</p></div></article></body>`

	// When the shared source selector is applied to HTTP markup.
	body, err := extractArticleText(markup, "bleepingcomputer")

	// Then only the class nested under the article element is returned.
	if err != nil || body != "Security article" {
		t.Fatalf("body = %q, err = %v", body, err)
	}
}
