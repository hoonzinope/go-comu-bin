package web

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderMarkdownAddsNewTabAttrsToExternalLinks(t *testing.T) {
	html := string(renderMarkdown("Read the [source](https://example.com/article)."))

	assert.Contains(t, html, `<a href="https://example.com/article" target="_blank" rel="noopener noreferrer nofollow">`)
	assert.Contains(t, html, `>source</a>`)
}

func TestRenderMarkdownLeavesRelativeLinksAlone(t *testing.T) {
	html := string(renderMarkdown("Jump to the [comments](#comments) or [post](/posts/123)."))

	assert.Contains(t, html, `<a href="#comments" rel="nofollow">`)
	assert.Contains(t, html, `<a href="/posts/123" rel="nofollow">`)
	assert.False(t, strings.Contains(html, `target="_blank"`))
}
