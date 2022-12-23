package bytedribble

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestParseMetainfo_Ubuntu(t *testing.T) {
	f, err := os.Open("testdata/ubuntu-22.04.1-live-server-amd64.iso.torrent")
	if err != nil {
		t.Fatal(err)
	}

	meta, err := ParseMetainfo(f)
	assert.NoError(t, err)
	t.Log(meta)
}
