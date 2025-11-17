package config

import (
	"testing"

	"yuanrong.org/kernel/runtime/faassdk/types"
)

func TestGetConfig(t *testing.T) {
	type args struct {
		funcSpec *types.FuncSpec
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"case1 succedd to get config", args{funcSpec: &types.FuncSpec{
			FuncMetaData: types.FuncMetaData{
				FunctionVersionURN: "sn:cn:yrk:12345678901234561234567890123456:function:0@yrservice@test-image-env",
			},
		}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetConfig(tt.args.funcSpec)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
