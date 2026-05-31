package htmlmd

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const leetcodeBaseURL = "https://leetcode.cn"

func Convert(rawHTML string) (string, error) {
	rawHTML = strings.TrimSpace(rawHTML)
	if rawHTML == "" {
		return "", nil
	}

	nodes, err := html.ParseFragment(strings.NewReader(rawHTML), &html.Node{
		Type:     html.ElementNode,
		DataAtom: atom.Body,
		Data:     "body",
	})
	if err != nil {
		return "", fmt.Errorf("parse html: %w", err)
	}

	var b markdownBuilder
	for _, node := range nodes {
		b.renderNode(node)
	}
	return cleanMarkdown(b.String()), nil
}

type markdownBuilder struct {
	bytes.Buffer
	listDepth int
}

func (b *markdownBuilder) renderNode(node *html.Node) {
	if node == nil {
		return
	}

	switch node.Type {
	case html.TextNode:
		b.WriteString(normalizeText(node.Data))
	case html.ElementNode:
		b.renderElement(node)
	default:
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			b.renderNode(child)
		}
	}
}

func (b *markdownBuilder) renderElement(node *html.Node) {
	switch strings.ToLower(node.Data) {
	case "p":
		b.ensureBlankLine()
		b.renderChildren(node)
		b.ensureBlankLine()
	case "br":
		b.WriteString("\n")
	case "strong", "b":
		b.WriteString("**")
		b.renderChildren(node)
		b.WriteString("**")
	case "em", "i":
		b.WriteString("*")
		b.renderChildren(node)
		b.WriteString("*")
	case "code":
		if isInside(node, "pre") {
			b.renderChildren(node)
			return
		}
		text := strings.TrimSpace(textContent(node))
		if text == "" {
			return
		}
		b.WriteString("`")
		b.WriteString(strings.ReplaceAll(text, "`", "\\`"))
		b.WriteString("`")
	case "pre":
		b.ensureBlankLine()
		lang := codeLanguage(node)
		b.WriteString("```")
		b.WriteString(lang)
		b.WriteString("\n")
		b.WriteString(strings.Trim(textContent(node), "\n\r\t "))
		b.WriteString("\n```\n")
		b.ensureBlankLine()
	case "ul":
		b.ensureLine()
		b.listDepth++
		b.renderChildren(node)
		b.listDepth--
		b.ensureLine()
	case "ol":
		b.ensureLine()
		b.listDepth++
		b.renderOrderedChildren(node)
		b.listDepth--
		b.ensureLine()
	case "li":
		b.ensureLine()
		b.WriteString(strings.Repeat("  ", max(0, b.listDepth-1)))
		b.WriteString("- ")
		b.renderChildren(node)
		b.ensureLine()
	case "h1", "h2", "h3", "h4", "h5", "h6":
		level := int(node.Data[1] - '0')
		b.ensureBlankLine()
		b.WriteString(strings.Repeat("#", level))
		b.WriteString(" ")
		b.renderChildren(node)
		b.ensureBlankLine()
	case "img":
		src := attr(node, "src")
		if src == "" {
			return
		}
		alt := strings.TrimSpace(attr(node, "alt"))
		b.WriteString("![")
		b.WriteString(escapeMarkdownLinkText(alt))
		b.WriteString("](")
		b.WriteString(markdownURL(src))
		b.WriteString(")")
	case "a":
		href := attr(node, "href")
		label := strings.TrimSpace(textContent(node))
		if href == "" || label == "" {
			b.renderChildren(node)
			return
		}
		b.WriteString("[")
		b.WriteString(escapeMarkdownLinkText(label))
		b.WriteString("](")
		b.WriteString(markdownURL(href))
		b.WriteString(")")
	case "table":
		b.ensureBlankLine()
		b.WriteString(htmlTableFallback(node))
		b.ensureBlankLine()
	default:
		b.renderChildren(node)
	}
}

func (b *markdownBuilder) renderChildren(node *html.Node) {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		b.renderNode(child)
	}
}

func (b *markdownBuilder) renderOrderedChildren(node *html.Node) {
	index := 1
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode || strings.ToLower(child.Data) != "li" {
			b.renderNode(child)
			continue
		}
		b.ensureLine()
		b.WriteString(strings.Repeat("  ", max(0, b.listDepth-1)))
		b.WriteString(fmt.Sprintf("%d. ", index))
		b.renderChildren(child)
		b.ensureLine()
		index++
	}
}

func (b *markdownBuilder) ensureLine() {
	text := b.String()
	if text == "" || strings.HasSuffix(text, "\n") {
		return
	}
	b.WriteString("\n")
}

func (b *markdownBuilder) ensureBlankLine() {
	text := b.String()
	if text == "" {
		return
	}
	if strings.HasSuffix(text, "\n\n") {
		return
	}
	if strings.HasSuffix(text, "\n") {
		b.WriteString("\n")
		return
	}
	b.WriteString("\n\n")
}

func cleanMarkdown(markdown string) string {
	lines := strings.Split(markdown, "\n")
	cleaned := make([]string, 0, len(lines))
	blank := 0
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		if strings.TrimSpace(line) == "" {
			blank++
			if blank > 1 {
				continue
			}
			cleaned = append(cleaned, "")
			continue
		}
		blank = 0
		cleaned = append(cleaned, line)
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n")) + "\n"
}

func normalizeText(text string) string {
	text = strings.ReplaceAll(text, "\u00a0", " ")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return text
}

func markdownURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	parsed, err := url.Parse(raw)
	if err == nil && parsed.IsAbs() {
		return raw
	}
	if strings.HasPrefix(raw, "//") {
		return "https:" + raw
	}
	if strings.HasPrefix(raw, "/") {
		return leetcodeBaseURL + raw
	}
	return raw
}

func escapeMarkdownLinkText(text string) string {
	text = strings.ReplaceAll(text, "[", "\\[")
	text = strings.ReplaceAll(text, "]", "\\]")
	return text
}

func attr(node *html.Node, key string) string {
	for _, attribute := range node.Attr {
		if strings.EqualFold(attribute.Key, key) {
			return attribute.Val
		}
	}
	return ""
}

func textContent(node *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current == nil {
			return
		}
		if current.Type == html.TextNode {
			b.WriteString(current.Data)
			return
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return b.String()
}

func codeLanguage(node *html.Node) string {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode || strings.ToLower(child.Data) != "code" {
			continue
		}
		className := attr(child, "class")
		for _, part := range strings.Fields(className) {
			if strings.HasPrefix(part, "language-") {
				return strings.TrimPrefix(part, "language-")
			}
		}
	}
	return ""
}

func isInside(node *html.Node, tag string) bool {
	for parent := node.Parent; parent != nil; parent = parent.Parent {
		if parent.Type == html.ElementNode && strings.EqualFold(parent.Data, tag) {
			return true
		}
	}
	return false
}

func htmlTableFallback(node *html.Node) string {
	var b strings.Builder
	b.WriteString("<table>")
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		renderRawHTML(&b, child)
	}
	b.WriteString("</table>")
	return b.String()
}

func renderRawHTML(b *strings.Builder, node *html.Node) {
	if node == nil {
		return
	}
	if err := html.Render(b, node); err != nil {
		return
	}
}
