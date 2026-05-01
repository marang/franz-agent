package jsonmerge

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMergeDocuments(t *testing.T) {
	t.Parallel()

	t.Run("deep merge with override", func(t *testing.T) {
		t.Parallel()

		got, err := MergeDocuments([][]byte{
			[]byte(`{"provider":{"a":1,"nested":{"x":1}}}`),
			[]byte(`{"provider":{"b":2,"nested":{"y":2}}}`),
			[]byte(`{"provider":{"nested":{}}}`),
		})
		require.NoError(t, err)
		require.JSONEq(t, `{"provider":{"a":1,"b":2,"nested":{"x":1,"y":2}}}`, string(got))
	})

	t.Run("non object top level fails", func(t *testing.T) {
		t.Parallel()

		_, err := MergeDocuments([][]byte{
			[]byte(`{"ok":true}`),
			[]byte(`[]`),
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "must be an object")
	})
}

func TestMergeReaders(t *testing.T) {
	t.Parallel()

	got, err := MergeReaders([]io.Reader{
		strings.NewReader(`{"a":1}`),
		strings.NewReader(`{"b":2}`),
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"a":1,"b":2}`, string(got))
}
