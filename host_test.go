package gohybrid

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/assert"
)

type testHandler struct {
	request  *http.Request
	response []byte
}

func (h *testHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.request = r
	w.Write(h.response)
}

func TestAPIGatewayProxyRequest(t *testing.T) {
	type testCase struct {
		Event                 interface{}
		Path                  string
		Method                string
		Body                  string
		Status                int
		QueryStringParameters map[string]string
	}

	cases := []testCase{
		{
			events.APIGatewayProxyRequest{
				HTTPMethod: "GET",
				Resource:   "/my/resource",
				Path:       "/my/resource",
				QueryStringParameters: map[string]string{
					"a": "b",
				},
				Body: "hello",
			},
			"/my/resource",
			"GET",
			"hello",
			200,
			map[string]string{"a": "b"},
		},
		{
			events.ALBTargetGroupRequest{
				HTTPMethod: "GET",
				Path:       "/my/resource",
				QueryStringParameters: map[string]string{
					"a": "b",
				},
				Body: "hello",
			},
			"/my/resource",
			"GET",
			"hello",
			200,
			map[string]string{"a": "b"},
		},
		{
			events.APIGatewayV2HTTPRequest{
				RawPath:        "/my/resource",
				RouteKey:       "DEFAULT",
				RawQueryString: "a=b",
				QueryStringParameters: map[string]string{
					"a": "b",
				},
				Body: "hello",
			},
			"/my/resource",
			"GET",
			"hello",
			200,
			map[string]string{"a": "b"},
		},
		{
			events.APIGatewayProxyRequest{
				HTTPMethod: "GET",
				Resource:   "/my/resource",
				Path:       "/my/resource",
				QueryStringParameters: map[string]string{
					"a": "b",
				},
				Body:            base64.StdEncoding.EncodeToString([]byte("hello")),
				IsBase64Encoded: true,
			},
			"/my/resource",
			"GET",
			"hello",
			200,
			map[string]string{"a": "b"},
		},
		{
			events.ALBTargetGroupRequest{
				HTTPMethod: "GET",
				Path:       "/my/resource",
				QueryStringParameters: map[string]string{
					"a": "b",
				},
				Body:            base64.StdEncoding.EncodeToString([]byte("hello")),
				IsBase64Encoded: true,
			},
			"/my/resource",
			"GET",
			"hello",
			200,
			map[string]string{"a": "b"},
		},
		{
			events.APIGatewayV2HTTPRequest{
				RawPath:        "/my/resource",
				RouteKey:       "DEFAULT",
				RawQueryString: "a=b",
				QueryStringParameters: map[string]string{
					"a": "b",
				},
				Body:            base64.StdEncoding.EncodeToString([]byte("hello")),
				IsBase64Encoded: true,
			},
			"/my/resource",
			"GET",
			"hello",
			200,
			map[string]string{"a": "b"},
		},
	}

	for _, c := range cases {
		// Arrange
		r, _ := json.Marshal(c.Event)
		th := &testHandler{}
		a := HttpAdapterHandler{
			http: th,
		}

		// Act
		_, err := a.Invoke(context.TODO(), r)
		assert.Nil(t, err)
		assert.Equal(t, c.Path, th.request.URL.Path)
		assert.Equal(t, c.Method, th.request.Method)
		b, _ := io.ReadAll(th.request.Body)
		assert.Equal(t, c.Body, string(b))
		for k, v := range c.QueryStringParameters {
			assert.Equal(t, v, th.request.URL.Query().Get(k))
		}
	}
}
