package corpus

import "strings"

// MaxChunkLen is the soft cap on chunk content length; sections longer than
// this are split on paragraph boundaries, each piece keeping its heading.
const MaxChunkLen = 2000

// SplitMarkdown divides a document into header-aware chunks (FR-4.5): each
// heading starts a new chunk holding the heading and its whole body, so
// logically coherent sections (a brief's risks, the criteria list) stay
// together. Oversized sections are split on paragraph boundaries with the
// heading repeated. Content before the first heading forms its own chunk.
func SplitMarkdown(docPath, content string) []Chunk {
	type section struct {
		heading string   // raw heading line, empty for the preamble
		trail   []string // heading texts from root to this section
		body    []string
	}

	sections := []*section{{}}
	cur := sections[0]
	var stack []struct {
		level int
		text  string
	}
	inFence := false

	for line := range strings.SplitSeq(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
		}
		if level, text, ok := parseHeading(trimmed); ok && !inFence {
			for len(stack) > 0 && stack[len(stack)-1].level >= level {
				stack = stack[:len(stack)-1]
			}
			stack = append(stack, struct {
				level int
				text  string
			}{level, text})

			trail := make([]string, len(stack))
			for i, h := range stack {
				trail[i] = h.text
			}
			cur = &section{heading: line, trail: trail}
			sections = append(sections, cur)
			continue
		}
		cur.body = append(cur.body, line)
	}

	var chunks []Chunk
	for _, s := range sections {
		label := strings.Join(s.trail, " > ")
		body := strings.TrimSpace(strings.Join(s.body, "\n"))
		if s.heading == "" && body == "" {
			continue
		}
		for _, piece := range packSection(s.heading, body) {
			chunks = append(chunks, Chunk{DocPath: docPath, Section: label, Content: piece})
		}
	}
	return chunks
}

// parseHeading reports whether line is an ATX heading, with its level and
// text.
func parseHeading(line string) (level int, text string, ok bool) {
	for level < len(line) && line[level] == '#' {
		level++
	}
	if level == 0 || level > 6 || level == len(line) || line[level] != ' ' {
		return 0, "", false
	}
	return level, strings.TrimSpace(line[level:]), true
}

// packSection renders a section as one chunk, or as several
// paragraph-aligned pieces — each repeating the heading — when it exceeds
// MaxChunkLen.
func packSection(heading, body string) []string {
	whole := body
	if heading != "" {
		whole = heading + "\n\n" + body
	}
	whole = strings.TrimSpace(whole)
	if whole == "" {
		return nil
	}
	if len(whole) <= MaxChunkLen {
		return []string{whole}
	}

	var pieces []string
	var sb strings.Builder
	flush := func() {
		if sb.Len() > 0 {
			pieces = append(pieces, strings.TrimSpace(sb.String()))
			sb.Reset()
		}
	}
	for para := range strings.SplitSeq(body, "\n\n") {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		if sb.Len() > 0 && sb.Len()+len(para) > MaxChunkLen {
			flush()
		}
		if sb.Len() == 0 && heading != "" {
			sb.WriteString(heading + "\n")
		}
		sb.WriteString("\n" + para + "\n")
	}
	flush()
	return pieces
}
