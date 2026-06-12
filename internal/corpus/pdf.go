package corpus

import (
	"bytes"
	"fmt"
	"io"

	"github.com/ledongthuc/pdf"
)

// ExtractPDF returns the plain text of the PDF at path, so exported notes
// and documents kept as PDF can join the corpus (FR-4.1). The underlying
// parser panics on some malformed files; that surfaces as an error here so
// one bad document cannot crash an ingestion run.
func ExtractPDF(path string) (text string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("extracting pdf %s: %v", path, r)
		}
	}()

	f, reader, err := pdf.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening pdf %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	plain, err := reader.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("extracting pdf %s: %w", path, err)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, plain); err != nil {
		return "", fmt.Errorf("extracting pdf %s: %w", path, err)
	}
	return buf.String(), nil
}
