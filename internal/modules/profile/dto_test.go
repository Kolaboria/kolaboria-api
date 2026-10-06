package profile

import (
	"encoding/json"
	"testing"
)

func TestUpdateProfileRequestDistinguishesOmittedNullAndValue(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantSet    bool
		wantNull   bool
		wantString string
	}{
		{name: "omitted", body: `{}`, wantSet: false},
		{name: "null clears", body: `{"bio":null}`, wantSet: true, wantNull: true},
		{name: "value updates", body: `{"bio":"Ringkasan"}`, wantSet: true, wantString: "Ringkasan"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var request UpdateProfileRequest
			if err := json.Unmarshal([]byte(test.body), &request); err != nil {
				t.Fatalf("unmarshal request: %v", err)
			}
			if request.Has("bio") != test.wantSet {
				t.Fatalf("Bio presence = %v, want %v", request.Has("bio"), test.wantSet)
			}
			if test.wantSet && (request.Bio == nil) != test.wantNull {
				t.Fatalf("Bio null = %v, want %v", request.Bio == nil, test.wantNull)
			}
			if request.Bio != nil && *request.Bio != test.wantString {
				t.Fatalf("Bio value = %q, want %q", *request.Bio, test.wantString)
			}
		})
	}
}
