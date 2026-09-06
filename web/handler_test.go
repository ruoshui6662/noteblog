package webassets

import (
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestStaticAndSPARoutes(t *testing.T) {
	handler := fileHandler(fstest.MapFS{
		"index.html":    {Data: []byte("<html>app</html>")},
		"assets/app.js": {Data: []byte("console.log('app')")},
	})
	for _, test := range []struct {
		path   string
		status int
	}{
		{"/", 200}, {"/assets/app.js", 200}, {"/docs/中文", 200},
		{"/admin/editor", 200}, {"/assets/missing.js", 404}, {"/unknown", 404},
	} {
		t.Run(test.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest("GET", test.path, nil))
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
		})
	}
}
