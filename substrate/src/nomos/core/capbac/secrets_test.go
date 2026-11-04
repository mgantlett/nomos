package capbac

import "testing"

func TestMatchSecretTokenLabel(t *testing.T) {
	awsKey := "AKIA" + "IOSFODNN7EXAMPLE"
	googleKey := "AIzaSy" + "DaGmWKa4JsXZ-HjGw7ISLn_3namBGewQe"

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "empty string",
			content: "",
			want:    "",
		},
		{
			name:    "standalone AWS access key in env assignment",
			content: "AWS_ACCESS_KEY_ID=" + awsKey,
			want:    "AWS access key",
		},
		{
			name:    "quoted AWS access key in JSON",
			content: `{"accessKeyId":"` + awsKey + `"}`,
			want:    "AWS access key",
		},
		{
			name:    "contiguous base64 binary chunk containing AKIA substring",
			content: "AAAAAKIAAAAEAO0AAZ8AAAAAAAAASAFKAKIASAFKAKQASAFKAKYASAFK",
			want:    "",
		},
		{
			name:    "quoted Google API key",
			content: `apiKey = "` + googleKey + `"`,
			want:    "Google API key",
		},
		{
			name:    "contiguous base64 chunk containing AIzaSy substring",
			content: "0123456789" + googleKey + "ABCDEFGH",
			want:    "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := MatchSecretTokenLabel(tc.content); got != tc.want {
				t.Fatalf("MatchSecretTokenLabel(%q) = %q, want %q", tc.content, got, tc.want)
			}
		})
	}
}
