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
	assert.Equal(t, "https://torrent.ubuntu.com/announce", meta.TrackerURL.String())
	assert.Equal(t, "ubuntu-22.04.1-live-server-amd64.iso", meta.Name)
	assert.Equal(t, 5627, len(meta.Hashes))
	assert.Equal(t, 1474873344, meta.TotalSizeBytes)
}
