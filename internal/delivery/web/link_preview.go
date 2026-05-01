package web

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	xhtml "golang.org/x/net/html"
)

const linkPreviewFetchTimeout = 3 * time.Second
const linkPreviewFetchLimit = 1 << 20

var fetchLinkPreviewMetadataFunc = fetchLinkPreviewMetadata

func extractLinkPreviewsFromMarkdown(ctx context.Context, value string) []LinkPreview {
	rendered := renderMarkdown(value)
	if rendered == "" {
		return nil
	}
	doc, err := xhtml.Parse(strings.NewReader(string(rendered)))
	if err != nil {
		return nil
	}
	seen := make(map[string]struct{})
	out := make([]LinkPreview, 0, 4)
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if node == nil {
			return
		}
		if node.Type == xhtml.ElementNode && node.Data == "a" {
			href := strings.TrimSpace(attrValue(node, "href"))
			if isExternalPreviewLink(href) {
				if _, ok := seen[href]; ok {
					goto children
				}
				title := strings.TrimSpace(nodeText(node))
				if title == "" {
					title = href
				}
				out = append(out, LinkPreview{
					Title: title,
					URL:   href,
					Host:  previewLinkHost(href),
				})
				seen[href] = struct{}{}
			}
		}
	children:
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return hydrateLinkPreviews(ctx, out)
}

func hydrateLinkPreviews(ctx context.Context, previews []LinkPreview) []LinkPreview {
	if len(previews) == 0 {
		return previews
	}
	out := make([]LinkPreview, len(previews))
	copy(out, previews)
	for i := range out {
		if !isExternalPreviewLink(out[i].URL) {
			continue
		}
		metadata := fetchLinkPreviewMetadataFunc(ctx, out[i].URL)
		if metadata.Title != "" {
			out[i].Title = metadata.Title
		}
		if metadata.Description != "" {
			out[i].Description = metadata.Description
		}
		if metadata.ImageURL != "" {
			out[i].ImageURL = metadata.ImageURL
		}
	}
	return out
}

type linkPreviewMetadata struct {
	Title       string
	Description string
	ImageURL    string
}

func fetchLinkPreviewMetadata(ctx context.Context, rawURL string) linkPreviewMetadata {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return linkPreviewMetadata{}
	}
	fetchCtx, cancel := context.WithTimeout(ctx, linkPreviewFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return linkPreviewMetadata{}
	}
	req.Header.Set("User-Agent", "Commu Bin Link Preview/1.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return linkPreviewMetadata{}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return linkPreviewMetadata{}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, linkPreviewFetchLimit))
	if err != nil {
		return linkPreviewMetadata{}
	}
	doc, err := xhtml.Parse(strings.NewReader(string(body)))
	if err != nil {
		return linkPreviewMetadata{}
	}

	meta := extractLinkPreviewMetadata(doc, parsed)
	return meta
}

func extractLinkPreviewMetadata(doc *xhtml.Node, baseURL *url.URL) linkPreviewMetadata {
	var meta linkPreviewMetadata
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if node == nil {
			return
		}
		if meta.Title == "" && node.Type == xhtml.ElementNode && node.Data == "title" {
			meta.Title = strings.TrimSpace(nodeText(node))
		}
		if node.Type == xhtml.ElementNode && node.Data == "meta" {
			property := strings.ToLower(strings.TrimSpace(attrValue(node, "property")))
			name := strings.ToLower(strings.TrimSpace(attrValue(node, "name")))
			content := strings.TrimSpace(attrValue(node, "content"))
			switch {
			case meta.Title == "" && content != "" && (property == "og:title" || name == "twitter:title"):
				meta.Title = content
			case meta.Description == "" && content != "" && (property == "og:description" || name == "twitter:description"):
				meta.Description = content
			case meta.ImageURL == "" && content != "" && (property == "og:image" || name == "twitter:image" || name == "twitter:image:src"):
				meta.ImageURL = resolvePreviewURL(baseURL, content)
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	meta.Title = normalizePreviewTitle(meta.Title)
	meta.Description = normalizePreviewText(meta.Description)
	meta.ImageURL = strings.TrimSpace(meta.ImageURL)
	return meta
}

func normalizePreviewTitle(value string) string {
	value = normalizePreviewText(value)
	if value == "" {
		return ""
	}
	return strings.TrimSpace(strings.ReplaceAll(value, "\n", " "))
}

func normalizePreviewText(value string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(value), " "))
}

func resolvePreviewURL(baseURL *url.URL, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if parsed.IsAbs() {
		return parsed.String()
	}
	if baseURL == nil {
		return raw
	}
	return baseURL.ResolveReference(parsed).String()
}

func isExternalPreviewLink(raw string) bool {
	if raw == "" {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return parsed.Host != ""
	default:
		return false
	}
}

func previewLinkHost(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if parsed.Host != "" {
		return parsed.Host
	}
	return raw
}

func attrValue(node *xhtml.Node, name string) string {
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, name) {
			return attr.Val
		}
	}
	return ""
}

func nodeText(node *xhtml.Node) string {
	if node == nil {
		return ""
	}
	var b strings.Builder
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n == nil {
			return
		}
		if n.Type == xhtml.TextNode {
			b.WriteString(n.Data)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return b.String()
}
