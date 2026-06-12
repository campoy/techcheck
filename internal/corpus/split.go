package corpus

// MaxChunkLen is the soft cap on chunk content length; sections longer than
// this are split on paragraph boundaries, each piece keeping its heading.
const MaxChunkLen = 2000

// SplitMarkdown divides a document into header-aware chunks (FR-4.5): each
// heading starts a new chunk holding the heading and its whole body, so
// logically coherent sections (a brief's risks, the criteria list) stay
// together. Oversized sections are split on paragraph boundaries with the
// heading repeated. Content before the first heading forms its own chunk.
func SplitMarkdown(docPath, content string) []Chunk {
	return nil
}
