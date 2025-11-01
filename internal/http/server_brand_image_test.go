package http

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/disintegration/imaging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_encodeBrandImage(t *testing.T) {
	t.Parallel()

	makeImage := func(width, height int) []byte {
		img := imaging.New(width, height, color.NRGBA{R: 100, G: 150, B: 200, A: 255})
		var buf bytes.Buffer
		require.NoError(t, png.Encode(&buf, img))
		return buf.Bytes()
	}

	type params struct {
		data     []byte
		maxBytes int64
	}
	type want struct {
		expectErr   error
		expectWidth int
	}

	tcs := []struct {
		name   string
		params params
		want   want
	}{
		{
			name: "resizes wider images to target width",
			params: params{
				data:     makeImage(1200, 600),
				maxBytes: 5 * 1024 * 1024,
			},
			want: want{
				expectWidth: brandImageTargetWidth,
			},
		},
		{
			name: "keeps original width when smaller",
			params: params{
				data:     makeImage(600, 400),
				maxBytes: 5 * 1024 * 1024,
			},
			want: want{
				expectWidth: 600,
			},
		},
		{
			name: "fails when exceeding max size",
			params: params{
				data:     makeImage(200, 200),
				maxBytes: 100,
			},
			want: want{
				expectErr: errBrandImageTooLarge,
			},
		},
		{
			name: "fails on invalid image data",
			params: params{
				data:     []byte("not-an-image"),
				maxBytes: 5 * 1024 * 1024,
			},
			want: want{
				expectErr: errBrandImageInvalid,
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := &Server{maxBrandImageBytes: tc.params.maxBytes}

			got, err := server.encodeBrandImage(bytes.NewReader(tc.params.data))
			if tc.want.expectErr != nil {
				require.Error(t, err, tc.name)
				assert.ErrorIs(t, err, tc.want.expectErr, tc.name)
				return
			}

			require.NoError(t, err, tc.name)
			assert.NotEmpty(t, got, tc.name)

			raw, err := base64.StdEncoding.DecodeString(got)
			require.NoError(t, err, tc.name)
			img, _, err := image.Decode(bytes.NewReader(raw))
			require.NoError(t, err, tc.name)
			assert.Equal(t, tc.want.expectWidth, img.Bounds().Dx(), tc.name)
		})
	}
}
