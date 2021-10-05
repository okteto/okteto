package okteto

import "testing"

func Test_IsTelemetryEnabled(t *testing.T) {
	var tests = []struct {
		name    string
		context *OktetoContext
		want    bool
	}{
		{name: "is-enabled", context: &OktetoContext{TelemetryEnabled: "true"}, want: true},
		{name: "is-disabled", context: &OktetoContext{TelemetryEnabled: "false"}, want: false},
		{name: "is-empty", context: &OktetoContext{TelemetryEnabled: ""}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			CurrentStore = &OktetoContextStore{
				CurrentContext: "test",
				Contexts: map[string]*OktetoContext{
					"test": tt.context,
				},
			}
			if got := IsTelemetryEnabled(); got != tt.want {
				t.Errorf("GetTelemetryEnabled, got %v, want %v", got, tt.want)
			}
		})
	}

}
