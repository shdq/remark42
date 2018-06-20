package proxy

import (
	"bytes"
	"image"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAvatarStore_Put(t *testing.T) {
	p := NewFSAvatarStore("/tmp/avatars.test", 300)
	err := os.MkdirAll("/tmp/avatars.test", 0700)
	require.NoError(t, err)
	defer os.RemoveAll("/tmp/avatars.test")

	avatar, err := p.Put("user1", nil)
	assert.Equal(t, "", avatar)
	assert.EqualError(t, err, "avatar reader is nil")

	avatar, err = p.Put("user1", strings.NewReader("some picture bin data"))
	require.Nil(t, err)
	assert.Equal(t, "b3daa77b4c04a9551b8781d03191fe098f325e67.image", avatar)
	fi, err := os.Stat("/tmp/avatars.test/30/b3daa77b4c04a9551b8781d03191fe098f325e67.image")
	assert.NoError(t, err)
	assert.Equal(t, int64(21), fi.Size())

	avatar, err = p.Put("user2", strings.NewReader("some picture bin data 123"))
	require.Nil(t, err)
	assert.Equal(t, "a1881c06eec96db9901c7bbfe41c42a3f08e9cb4.image", avatar)
	fi, err = os.Stat("/tmp/avatars.test/84/a1881c06eec96db9901c7bbfe41c42a3f08e9cb4.image")
	assert.NoError(t, err)
	assert.Equal(t, int64(25), fi.Size())

	// with resize
	file, e := os.Open("testdata/circles.png")
	require.Nil(t, e)
	avatar, err = p.Put("user3", file)
	require.Nil(t, err)
	assert.Equal(t, "0b7f849446d3383546d15a480966084442cd2193.image", avatar)
	fi, err = os.Stat("/tmp/avatars.test/60/0b7f849446d3383546d15a480966084442cd2193.image")
	assert.NoError(t, err)
	assert.Equal(t, int64(6986), fi.Size())

	p = NewFSAvatarStore("/dev/null", 300)
	_, err = p.Put("user1", strings.NewReader("some picture bin data"))
	assert.EqualError(t, err, "can't create file /dev/null/30/b3daa77b4c04a9551b8781d03191fe098f325e67.image: open /dev/null/30/b3daa77b4c04a9551b8781d03191fe098f325e67.image: not a directory")
}

func TestAvatarStore_Get(t *testing.T) {
	p := NewFSAvatarStore("/tmp/avatars.test", 300)
	err := os.MkdirAll("/tmp/avatars.test/30", 0700)
	require.NoError(t, err)
	defer os.RemoveAll("/tmp/avatars.test")

	// file not exists
	r, size, err := p.Get("some_random_name.image")
	// nil, 0, errors.Wrapf(err, "can't load avatar %s, id")
	assert.Nil(t, r)
	assert.Equal(t, 0, size)
	assert.EqualError(t, err, "can't load avatar some_random_name.image, id: open /tmp/avatars.test/91/some_random_name.image: no such file or directory")
	// file exists
	err = ioutil.WriteFile("/tmp/avatars.test/30/b3daa77b4c04a9551b8781d03191fe098f325e67.image", []byte("something"), 0666)
	assert.Nil(t, err)
	r, size, err = p.Get("b3daa77b4c04a9551b8781d03191fe098f325e67.image")

	assert.Nil(t, err)
	assert.Equal(t, 9, size)
	data, err := ioutil.ReadAll(r)
	assert.Nil(t, err)
	assert.Equal(t, "something", string(data))
}

func TestAvatarStore_Location(t *testing.T) {
	p := NewFSAvatarStore("/tmp/avatars.test", 300)

	tbl := []struct {
		id  string
		res string
	}{
		{"abc", "/tmp/avatars.test/35"},
		{"xyz", "/tmp/avatars.test/69"},
		{"blah blah", "/tmp/avatars.test/29"},
	}

	for i, tt := range tbl {
		assert.Equal(t, tt.res, p.location(tt.id), "test #%d", i)
	}
}

func TestAvatarStore_resize(t *testing.T) {
	checkC := func(t *testing.T, r io.Reader, cExp []byte) {
		content, err := ioutil.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, cExp, content)
	}

	// Reader is nil.
	resizedR := resize(nil, 100)
	// assert.EqualError(t, err, "limit should be greater than 0")
	assert.Nil(t, resizedR)

	// Negative limit error.
	resizedR = resize(strings.NewReader("some picture bin data"), -1)
	require.NotNil(t, resizedR)
	checkC(t, resizedR, []byte("some picture bin data"))

	// Decode error.
	resizedR = resize(strings.NewReader("invalid image content"), 100)
	assert.NotNil(t, resizedR)
	checkC(t, resizedR, []byte("invalid image content"))

	cases := []struct {
		file   string
		wr, hr int
	}{
		{"testdata/circles.png", 400, 300}, // full size: 800x600 px
		{"testdata/circles.jpg", 300, 400}, // full size: 600x800 px
	}

	for _, c := range cases {
		img, err := ioutil.ReadFile(c.file)
		require.Nil(t, err, "can't open test file %s", c.file)

		// No need for resize, avatar dimensions are smaller than resize limit.
		resizedR = resize(bytes.NewReader(img), 800)
		assert.NotNilf(t, resizedR, "file %s", c.file)
		checkC(t, resizedR, img)

		// Resizing to half of width. Check resizedR avatar format PNG.
		resizedR = resize(bytes.NewReader(img), 400)
		assert.NotNilf(t, resizedR, "file %s", c.file)

		imgRz, format, err := image.Decode(resizedR)
		assert.Nilf(t, err, "file %s", c.file)
		assert.Equalf(t, "png", format, "file %s", c.file)
		bounds := imgRz.Bounds()
		assert.Equalf(t, c.wr, bounds.Dx(), "file %s", c.file)
		assert.Equalf(t, c.hr, bounds.Dy(), "file %s", c.file)
	}
}

func TestAvatarStore_ID(t *testing.T) {
	p := NewFSAvatarStore("/tmp/avatars.test", 300)
	err := os.MkdirAll("/tmp/avatars.test/30", 0700)
	require.NoError(t, err)
	defer os.RemoveAll("/tmp/avatars.test")

	// file not exists
	id := p.ID("some_random_name.image")
	assert.Equal(t, "a008de0a2ccb3308b5d99ffff66436e15538f701", id) // store.EncodeID("some_random_name.image")
	// file exists
	err = ioutil.WriteFile("/tmp/avatars.test/30/b3daa77b4c04a9551b8781d03191fe098f325e67.image", []byte("something"), 0666)
	require.NoError(t, err)
	touch := time.Date(2017, 7, 14, 2, 40, 0, 0, time.UTC) // 1500000000
	err = os.Chtimes("/tmp/avatars.test/30/b3daa77b4c04a9551b8781d03191fe098f325e67.image", touch, touch)
	require.NoError(t, err)
	id = p.ID("b3daa77b4c04a9551b8781d03191fe098f325e67.image")
	assert.Equal(t, "325d5b451f32c2f8e7f30a9fd65bff6a42954d9a", id) // store.EncodeID("b3daa77b4c04a9551b8781d03191fe098f325e67.image1500000000")
}
func BenchmarkAvatarStore_ID(b *testing.B) {
	p := NewFSAvatarStore("/tmp/avatars.test", 300)
	os.MkdirAll("/tmp/avatars.test/30", 0700)
	defer os.RemoveAll("/tmp/avatars.test")
	err := ioutil.WriteFile("/tmp/avatars.test/30/b3daa77b4c04a9551b8781d03191fe098f325e67.image", []byte("something"), 0666)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.ID("b3daa77b4c04a9551b8781d03191fe098f325e67.image")
	}
}
