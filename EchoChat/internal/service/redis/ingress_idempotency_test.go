package redis

import "testing"

func TestLuaResultString(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		want    string
		wantErr bool
	}{
		{
			name:  "nil",
			input: nil,
			want:  "",
		},
		{
			name:  "string",
			input: "DONE",
			want:  "DONE",
		},
		{
			name:  "bytes",
			input: []byte("PENDING"),
			want:  "PENDING",
		},
		{
			name:    "unexpected type",
			input:   123,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := luaResultString(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %T", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("luaResultString returned error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("luaResultString = %q, want %q", got, tc.want)
			}
		})
	}
}
