package pan

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-resty/resty/v2"
)

type accountShapeWire struct {
	apiResponse
	Data struct {
		Name string `json:"user_name"`
	} `json:"data"`
}

// Error envelopes return "data": [] with state=false; apiRequest must surface
// the API error itself instead of failing to decode the payload.
func TestAPIRequestSurfacesErrorEnvelope(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"state":false,"code":770004,"message":"已达到当前访问上限，请稍后再试","data":[]}`))
	}))
	defer ts.Close()
	client := &Client{http: resty.New()}
	_, err := apiRequest[accountShapeWire](client, client.http.R().SetContext(context.Background()), http.MethodGet, ts.URL, "account")
	var apiErr *apiError
	if !errors.As(err, &apiErr) || apiErr.Code != 770004 {
		t.Fatalf("expected apiError 770004, got %v", err)
	}
}

func TestAPIRequestDecodesSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"state":true,"code":0,"data":{"user_name":"jemy"}}`))
	}))
	defer ts.Close()
	client := &Client{http: resty.New()}
	result, err := apiRequest[accountShapeWire](client, client.http.R().SetContext(context.Background()), http.MethodGet, ts.URL, "account")
	if err != nil || result.Data.Name != "jemy" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}
