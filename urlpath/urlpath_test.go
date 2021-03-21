package urlpath

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPopLeft(t *testing.T) {
	table := []struct {
		Path      string
		LeftSeg   string
		RightPath string
	}{
		{Path: "/a/b/c", LeftSeg: "a", RightPath: "/b/c"},
		{Path: "//a/b/c", LeftSeg: "a", RightPath: "/b/c"},
		{Path: "/a//b/c", LeftSeg: "a", RightPath: "//b/c"},
		{Path: "/c", LeftSeg: "c", RightPath: ""},
		{Path: "/", LeftSeg: "", RightPath: ""},
		{Path: "a/b/c", LeftSeg: "a", RightPath: "b/c"},
		{Path: "a//b/c", LeftSeg: "a", RightPath: "b/c"},
		{Path: "c", LeftSeg: "c", RightPath: ""},
		{Path: "", LeftSeg: "", RightPath: ""},
	}

	for _, row := range table {
		t.Run("Path:"+row.Path, func(t *testing.T) {
			leftSeg, rightPath := PopLeft(row.Path)
			require.Equal(t, row.LeftSeg, leftSeg)
			require.Equal(t, row.RightPath, rightPath)
		})
	}

}
