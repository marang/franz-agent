package jsonmerge

import (
	"encoding/json"
	"fmt"
	"io"
)

// MergeDocuments deep-merges JSON object documents in order.
// Later documents override earlier values.
func MergeDocuments(docs [][]byte) ([]byte, error) {
	merged := map[string]any{}

	for i, doc := range docs {
		if len(doc) == 0 {
			continue
		}

		var parsed any
		if err := json.Unmarshal(doc, &parsed); err != nil {
			return nil, fmt.Errorf("invalid JSON document %d: %w", i, err)
		}

		obj, ok := parsed.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("JSON document %d must be an object", i)
		}

		mergeObject(merged, obj)
	}

	return json.Marshal(merged)
}

// MergeReaders reads and merges JSON object documents from readers.
func MergeReaders(readers []io.Reader) ([]byte, error) {
	docs := make([][]byte, 0, len(readers))
	for i, r := range readers {
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("failed to read JSON document %d: %w", i, err)
		}
		docs = append(docs, data)
	}
	return MergeDocuments(docs)
}

func mergeObject(dst, src map[string]any) {
	for key, srcValue := range src {
		srcMap, srcIsMap := srcValue.(map[string]any)
		if !srcIsMap {
			dst[key] = srcValue
			continue
		}

		if dstValue, ok := dst[key]; ok {
			if dstMap, ok := dstValue.(map[string]any); ok {
				mergeObject(dstMap, srcMap)
				dst[key] = dstMap
				continue
			}
		}

		dst[key] = cloneMap(srcMap)
	}
}

func cloneMap(src map[string]any) map[string]any {
	out := make(map[string]any, len(src))
	for key, value := range src {
		if nested, ok := value.(map[string]any); ok {
			out[key] = cloneMap(nested)
			continue
		}
		out[key] = value
	}
	return out
}
