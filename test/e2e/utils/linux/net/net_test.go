package net

import (
	"net"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseProcNetEntries(t *testing.T) {
	tt := []struct {
		name            string
		procNetString   string
		expectedEntries ProcNetEntries
		expectedErr     error
	}{
		{
			name: "valid entries",
			procNetString: strings.TrimLeft(`
   0: 00000000:1F6B 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 9979636 1 0000000000000000 100 0 0 10 0                   
   1: 0100007F:2329 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 9977307 1 0000000000000000 100 0 0 10 0                   
  11: 0100007F:A6B6 0100007F:2710 06 00000000:00000000 03:000016E6 00000000     0        0 0 3 0000000000000000                                      
  12: 0100007F:C648 0100007F:2710 01 00000000:00000000 02:00000855 00000000     0        0 10404562 2 0000000000000000 20 4 20 10 -1                 
  68: 9700550A:81AC 0100600A:01BB 01 00000000:00000000 02:000003F6 00000000     0        0 9976360 2 0000000000000000 20 4 30 10 -1                  
   0: 00000000000000000000000000000000:2711 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 9977447 1 0000000000000000 100 0 0 10 0
   2: 0000000000000000FFFF00000100007F:1C1F 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 9976573 1 0000000000000000 100 0 0 10 0
   7: 0000000000000000FFFF00009700550A:1F90 0000000000000000FFFF00000100550A:8F3E 06 00000000:00000000 03:0000035E 00000000     0        0 0 3 0000000000000000
   8: 0000000000000000FFFF00009700550A:2711 0000000000000000FFFF00005100550A:929C 01 00000000:00000000 02:00000404 00000000     0        0 10355352 2 0000000000000000 21 4 15 10 -1
`, "\n"),
			expectedEntries: ProcNetEntries{
				{
					LocalAddress: AddressPort{
						Address: net.ParseIP("0.0.0.0"),
						Port:    8043,
					},
					RemoteAddress: AddressPort{
						Address: net.ParseIP("0.0.0.0"),
						Port:    0,
					},
					State: EntryStateListen,
				},
				{
					LocalAddress: AddressPort{
						Address: net.ParseIP("127.0.0.1"),
						Port:    9001,
					},
					RemoteAddress: AddressPort{
						Address: net.ParseIP("0.0.0.0"),
						Port:    0,
					},
					State: EntryStateListen,
				},
				{
					LocalAddress: AddressPort{
						Address: net.ParseIP("127.0.0.1"),
						Port:    42678,
					},
					RemoteAddress: AddressPort{
						Address: net.ParseIP("127.0.0.1"),
						Port:    10000,
					},
					State: EntryStateTimeWait,
				},
				{
					LocalAddress: AddressPort{
						Address: net.ParseIP("127.0.0.1"),
						Port:    50760,
					},
					RemoteAddress: AddressPort{
						Address: net.ParseIP("127.0.0.1"),
						Port:    10000,
					},
					State: EntryStateEstablished,
				},
				{
					LocalAddress: AddressPort{
						Address: net.ParseIP("10.85.0.151"),
						Port:    33196,
					},
					RemoteAddress: AddressPort{
						Address: net.ParseIP("10.96.0.1"),
						Port:    443,
					},
					State: EntryStateEstablished,
				},
				{
					LocalAddress: AddressPort{
						Address: net.ParseIP("::"),
						Port:    10001,
					},
					RemoteAddress: AddressPort{
						Address: net.ParseIP("::"),
						Port:    0,
					},
					State: EntryStateListen,
				},
				{
					LocalAddress: AddressPort{
						Address: net.ParseIP("7f00:1:0:ffff::"),
						Port:    7199,
					},
					RemoteAddress: AddressPort{
						Address: net.ParseIP("::"),
						Port:    0,
					},
					State: EntryStateListen,
				},
				{
					LocalAddress: AddressPort{
						Address: net.ParseIP("a55:97:0:ffff::"),
						Port:    8080,
					},
					RemoteAddress: AddressPort{
						Address: net.ParseIP("a55:1:0:ffff::"),
						Port:    36670,
					},
					State: EntryStateTimeWait,
				},
				{
					LocalAddress: AddressPort{
						Address: net.ParseIP("a55:97:0:ffff::"),
						Port:    10001,
					},
					RemoteAddress: AddressPort{
						Address: net.ParseIP("a55:51:0:ffff::"),
						Port:    37532,
					},
					State: EntryStateEstablished,
				},
			},
		},
	}

	for i := range tt {
		tc := &tt[i]
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseProcNetEntries(tc.procNetString)

			if !reflect.DeepEqual(tc.expectedErr, err) {
				t.Errorf("expected and got error differ: %s", cmp.Diff(tc.expectedErr, err))
			}

			if !reflect.DeepEqual(tc.expectedEntries, got) {
				t.Errorf("expected and got entries differ: %s", cmp.Diff(tc.expectedEntries, got))
			}
		})
	}
}
