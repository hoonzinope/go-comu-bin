package web

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractLinkPreviewsFromMarkdownReturnsExternalLinksOnly(t *testing.T) {
	orig := fetchLinkPreviewMetadataFunc
	fetchLinkPreviewMetadataFunc = func(ctx context.Context, rawURL string) linkPreviewMetadata {
		_ = ctx
		_ = rawURL
		return linkPreviewMetadata{Title: "OG title", Description: "OG description", ImageURL: "https://example.com/image.jpg"}
	}
	t.Cleanup(func() { fetchLinkPreviewMetadataFunc = orig })

	links := extractLinkPreviewsFromMarkdown(context.Background(), "Intro [docs](https://example.com/docs) and [jump](#comments) plus [blog](https://blog.example.org/path).")

	require.Len(t, links, 2)
	assert.Equal(t, "OG title", links[0].Title)
	assert.Equal(t, "OG description", links[0].Description)
	assert.Equal(t, "https://example.com/image.jpg", links[0].ImageURL)
	assert.Equal(t, "example.com", links[0].Host)
	assert.Equal(t, "https://example.com/docs", links[0].URL)
	assert.Equal(t, "OG title", links[1].Title)
	assert.Equal(t, "blog.example.org", links[1].Host)
	assert.Equal(t, "https://blog.example.org/path", links[1].URL)
}

func TestExtractLinkPreviewsFromMarkdownDeduplicatesRepeatedLinks(t *testing.T) {
	orig := fetchLinkPreviewMetadataFunc
	fetchLinkPreviewMetadataFunc = func(ctx context.Context, rawURL string) linkPreviewMetadata {
		_ = ctx
		_ = rawURL
		return linkPreviewMetadata{}
	}
	t.Cleanup(func() { fetchLinkPreviewMetadataFunc = orig })

	links := extractLinkPreviewsFromMarkdown(context.Background(), "[source](https://example.com) and again [source](https://example.com)")

	require.Len(t, links, 1)
	assert.Equal(t, "source", links[0].Title)
}

func TestExtractLinkPreviewsFromMarkdownDeduplicatesCanonicalVariants(t *testing.T) {
	orig := fetchLinkPreviewMetadataFunc
	fetchLinkPreviewMetadataFunc = func(ctx context.Context, rawURL string) linkPreviewMetadata {
		_ = ctx
		_ = rawURL
		return linkPreviewMetadata{CanonicalURL: "https://example.com/2026/04/29/uber-is-in-the-hotel-business-now-thanks-in-part-to-ai/"}
	}
	t.Cleanup(func() { fetchLinkPreviewMetadataFunc = orig })

	links := extractLinkPreviewsFromMarkdown(context.Background(), "[source](https://example.com/2026/4/29/uber-is-in-the-hotel-business-now-thanks-in-part-to-ai/) and [source](https://example.com/2026/04/29/uber-is-in-the-hotel-business-now-thanks-in-part-to-ai/)")

	require.Len(t, links, 1)
	assert.Equal(t, "https://example.com/2026/4/29/uber-is-in-the-hotel-business-now-thanks-in-part-to-ai/", links[0].URL)
	assert.Equal(t, "https://example.com/2026/04/29/uber-is-in-the-hotel-business-now-thanks-in-part-to-ai/", links[0].CanonicalURL)
}
