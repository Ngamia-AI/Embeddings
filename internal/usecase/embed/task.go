package embed

import (
	"github.com/camel-creatives/embeddings-api/internal/domain"
)

// ModelID is the public model identifier.
const ModelID = "embeddinggemma-2"

// FullDimensions is the model's native vector size.
const FullDimensions = 768

// SupportedDimensions are the truncation targets.
var SupportedDimensions = []int{768, 512, 256, 128}

// Token costs used for the pre-engine budget estimate.
const (
	TokensPerImage    = 280
	TokensPerAudioSec = 25
	TokensPerVideoSec = 140 // one frame per second
	bytesPerTextToken = 4
)

// taskPrefixes maps the rich task field to the model's instruction prefix.
var taskPrefixes = map[string]string{
	"search_query":       " task: search result | query: ",
	"question_answering": " task: question answering | query: ",
	"fact_checking":      " task: fact checking | query: ",
	"code_retrieval":     " task: code retrieval | query: ",
	"classification":     " task: classification | query: ",
	"clustering":         " task: clustering | query: ",
	"similarity":         " task: sentence similarity | query: ",
}

// validDimension reports whether d is 0 (native) or a supported truncation.
func validDimension(d int) bool {
	if d == 0 || d == FullDimensions {
		return true
	}
	for _, s := range SupportedDimensions {
		if d == s {
			return true
		}
	}
	return false
}

// textPrefix returns the instruction prefix for a text part, or "" for none.
// Task prefixes apply to text only; media is never prefixed.
func textPrefix(task, inputType, title string) string {
	if p, ok := taskPrefixes[task]; ok && task != "" {
		return p
	}
	switch inputType {
	case "query", "search_query":
		return taskPrefixes["search_query"]
	case "document":
		return documentPrefix(title)
	default:
		return ""
	}
}

func documentPrefix(title string) string {
	if title == "" {
		title = "none"
	}
	return " title: " + title + " | text: "
}

// modalityOf returns a representative modality label for metrics.
func modalityOf(u domain.Usage) string {
	switch {
	case u.VideoSeconds > 0:
		return "video"
	case u.AudioSeconds > 0:
		return "audio"
	case u.Images > 0:
		return "image"
	default:
		return "text"
	}
}
