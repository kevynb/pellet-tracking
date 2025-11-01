package http

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/disintegration/imaging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pellets-tracker/internal/core"
)

func TestServer_handleBrandsPage(t *testing.T) {
	t.Parallel()

	makeImage := func(width, height int) []byte {
		img := imaging.New(width, height, color.NRGBA{R: 20, G: 40, B: 60, A: 255})
		var buf bytes.Buffer
		require.NoError(t, png.Encode(&buf, img))
		return buf.Bytes()
	}

	type params struct {
		imageData []byte
		maxBytes  int64
	}
	type want struct {
		statusCode      int
		expectRedirect  bool
		expectErrorText string
		expectedWidth   int
	}

	tcs := []struct {
		name   string
		params params
		want   want
	}{
		{
			name: "uploads image and resizes",
			params: params{
				imageData: makeImage(1200, 600),
				maxBytes:  5 * 1024 * 1024,
			},
			want: want{
				statusCode:     http.StatusSeeOther,
				expectRedirect: true,
				expectedWidth:  brandImageTargetWidth,
			},
		},
		{
			name: "rejects oversized upload",
			params: params{
				imageData: makeImage(400, 400),
				maxBytes:  100,
			},
			want: want{
				statusCode:      http.StatusBadRequest,
				expectErrorText: "Image trop volumineuse",
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := &stubDataStore{}

			server := NewServer(store, Config{MaxBrandImageBytes: tc.params.maxBytes})

			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			require.NoError(t, writer.WriteField("name", "Bois & Co"))
			require.NoError(t, writer.WriteField("description", "Pellets premium"))
			if len(tc.params.imageData) > 0 {
				part, err := writer.CreateFormFile("image_file", "brand.png")
				require.NoError(t, err)
				_, err = part.Write(tc.params.imageData)
				require.NoError(t, err)
			}
			require.NoError(t, writer.Close())

			req := httptest.NewRequest(http.MethodPost, "/marques", body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			rec := httptest.NewRecorder()

			server.handleBrandsPage(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			assert.Equal(t, tc.want.statusCode, res.StatusCode, tc.name)
			if tc.want.expectRedirect {
				assert.Equal(t, "/marques?added=brand", res.Header.Get("Location"), tc.name)
				require.True(t, store.replaced, tc.name)
				require.Len(t, store.replacedWith.Brands, 1, tc.name)
				if tc.want.expectedWidth > 0 {
					raw, err := base64.StdEncoding.DecodeString(store.replacedWith.Brands[0].ImageBase64)
					require.NoError(t, err, tc.name)
					img, _, err := image.Decode(bytes.NewReader(raw))
					require.NoError(t, err, tc.name)
					assert.Equal(t, tc.want.expectedWidth, img.Bounds().Dx(), tc.name)
				}
			} else {
				assert.False(t, store.replaced, tc.name)
				if tc.want.expectErrorText != "" {
					responseBody := rec.Body.String()
					assert.Contains(t, responseBody, tc.want.expectErrorText, tc.name)
				}
			}
		})
	}
}

type stubDataStore struct {
	data         core.DataStore
	replaced     bool
	replacedWith core.DataStore
}

func (s *stubDataStore) Data() core.DataStore {
	return s.data
}

func (s *stubDataStore) Replace(ds core.DataStore) error {
	s.replaced = true
	s.replacedWith = ds
	s.data = ds
	return nil
}
