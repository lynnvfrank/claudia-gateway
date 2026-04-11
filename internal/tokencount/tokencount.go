// Package tokencount provides fast local token counts via tiktoken (cl100k_base).
// Counts may differ from the upstream provider when the routed model uses another tokenizer.
package tokencount

import (
	"sync"

	"github.com/pkoukk/tiktoken-go"
)

const encodingName = "cl100k_base"

var (
	encOnce sync.Once
	enc     *tiktoken.Tiktoken
	encErr  error
)

func encoding() (*tiktoken.Tiktoken, error) {
	encOnce.Do(func() {
		enc, encErr = tiktoken.GetEncoding(encodingName)
	})
	return enc, encErr
}

// Count returns the cl100k_base token count for s (ordinary BPE; no special-token handling).
func Count(s string) (int, error) {
	e, err := encoding()
	if err != nil {
		return 0, err
	}
	return len(e.EncodeOrdinary(s)), nil
}
