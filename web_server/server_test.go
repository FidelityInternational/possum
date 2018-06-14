package webServer_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
)

var (
	serverMux *http.ServeMux
)

type MockRoute struct {
	Method       string
	Endpoint     string
	FirstOutput  string
	SecondOutput string
	OutputNumber int
}

func setup(mock MockRoute) *httptest.Server {
	return setupMultiple([]MockRoute{mock})
}

func setupMultiple(mockEndpoints []MockRoute) *httptest.Server {
	serverMux = http.NewServeMux()
	newServer := httptest.NewServer(serverMux)
	m := martini.New()
	m.Use(render.Renderer())
	r := martini.NewRouter()
	for _, mock := range mockEndpoints {
		var output string
		method := mock.Method
		endpoint := mock.Endpoint
		outputNumber := mock.OutputNumber
		if outputNumber == 0 {
			output = mock.FirstOutput
		} else {
			output = mock.SecondOutput
		}
		if method == "GET" {
			r.Get(endpoint, func() string {
				if outputNumber == 0 {
					mock.OutputNumber++
					outputNumber++
					return output
				} else {
					return mock.SecondOutput
				}
			})
		} else if method == "POST" {
			r.Post(endpoint, func() string {
				return output
			})
		} else if method == "DELETE" {
			r.Delete(endpoint, func() (int, string) {
				return 204, output
			})
		}
	}

	m.Action(r.Handle)
	serverMux.Handle("/", m)
	return newServer
}

func teardown(server *httptest.Server) {
	server.Close()
}
