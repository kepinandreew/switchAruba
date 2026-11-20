package aruba

import "testing"

func TestLookupVendor(t *testing.T) {
	cases := []struct {
		mac  string
		want string
	}{
		{"00:1d:c1:12:34:56", "Audinate (Dante)"},
		{"001d.c1aa.bbcc", "Audinate (Dante)"},
		{"00-a0-de-11-22-33", "Yamaha"},
		{"00:0e:dd:01:02:03", "Shure"},
		{"00:60:74:ab:cd:ef", "QSC / Q-SYS"},
		{"00:04:c4:00:11:22", "Allen & Heath"},
		{"78:45:01:aa:bb:cc", "Biamp"},
		{"7c:2e:0d:11:22:33", "Blackmagic Design"},
		{"94:05:bb:31:22:33", "Neutrik"},
		{"54:b5:6c:01:02:03", "NovaStar"},
		{"00:10:7f:aa:bb:cc", "Crestron"},
		{"00:0b:86:aa:bb:cc", "HPE / Aruba"},
		{"24:de:c6:11:22:33", "HPE / Aruba"},
		{"24:48:45:11:22:33", "Hikvision"},
		{"00:40:8c:aa:bb:cc", "Axis"},
		{"00:03:93:11:22:33", "Apple"},
		{"00:06:5b:11:22:33", "Dell"},
		{"02:aa:bb:cc:dd:ee", "Private / Randomized"},
		{"not-a-mac", ""},
	}
	for _, tc := range cases {
		got := LookupVendor(tc.mac)
		if got != tc.want {
			t.Errorf("LookupVendor(%q) = %q, want %q", tc.mac, got, tc.want)
		}
	}
	if len(ouiDB) < 35000 {
		t.Fatalf("expected embedded IEEE OUI table >= 35000 entries, got %d", len(ouiDB))
	}
}
