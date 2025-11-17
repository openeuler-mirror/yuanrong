package fast

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
	"testing"

	"yuanrong/pkg/common/httputil/http"
	"yuanrong/pkg/common/snerror"
)

func Test_parseFastResponse(t *testing.T) {
	response1 := &fasthttp.Response{}
	badResponse := snerror.BadResponse{
		Code:    0,
		Message: "500 error",
	}
	bytes, _ := json.Marshal(badResponse)
	response1.SetStatusCode(fasthttp.StatusInternalServerError)
	response1.SetBody(bytes)

	response2 := &fasthttp.Response{}

	response3 := &fasthttp.Response{}
	response3.SetStatusCode(fasthttp.StatusBadRequest)

	tests := []struct {
		name     string
		response *fasthttp.Response
		want     *http.SuccessResponse
		wantErr  bool
	}{
		{
			name:     "test 500",
			response: response1,
			wantErr:  true,
		},
		{
			name:     "test 200",
			response: response2,
			wantErr:  false,
		},
		{
			name:     "test 400",
			response: response3,
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseFastResponse(tt.response)
			assert.Equal(t, err != nil, tt.wantErr)
		})
	}
}
