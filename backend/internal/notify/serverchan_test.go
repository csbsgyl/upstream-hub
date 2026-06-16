package notify

import "testing"

func TestResolveServerChanURL(t *testing.T) {
	cases := []struct {
		name    string
		sendkey string
		apiBase string
		want    string
	}{
		{
			name:    "turbo key",
			sendkey: "SCT123456ABCdef",
			want:    "https://sctapi.ftqq.com/SCT123456ABCdef.send",
		},
		{
			name:    "serverchan3 sctp key",
			sendkey: "sctp1234tABCDEFxyz",
			want:    "https://1234.push.ft07.com/send/sctp1234tABCDEFxyz.send",
		},
		{
			name:    "custom api base trims trailing slash",
			sendkey: "anything",
			apiBase: "https://push.example.com/",
			want:    "https://push.example.com/anything.send",
		},
		{
			name:    "custom api base overrides sctp detection",
			sendkey: "sctp99tZZ",
			apiBase: "https://self.host",
			want:    "https://self.host/sctp99tZZ.send",
		},
		{
			name:    "sctp prefix without digits falls back to turbo",
			sendkey: "sctpfoo",
			want:    "https://sctapi.ftqq.com/sctpfoo.send",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveServerChanURL(tc.sendkey, tc.apiBase)
			if got != tc.want {
				t.Errorf("resolveServerChanURL(%q, %q) = %q, want %q", tc.sendkey, tc.apiBase, got, tc.want)
			}
		})
	}
}
