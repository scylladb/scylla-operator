// Copyright (C) 2017 ScyllaDB

package yaml

import (
	"bytes"
	"strings"
	"testing"
)

func TestToUnstructured(t *testing.T) {
	type args struct {
		rawyaml []byte
	}
	tests := []struct {
		name          string
		args          args
		wantObjsCount int
		err           string
	}{
		{
			name: "single object",
			args: args{
				rawyaml: []byte("apiVersion: v1\n" +
					"kind: ConfigMap\n"),
			},
			wantObjsCount: 1,
		},
		{
			name: "multiple objects are detected",
			args: args{
				rawyaml: []byte("apiVersion: v1\n" +
					"kind: ConfigMap\n" +
					"---\n" +
					"apiVersion: v1\n" +
					"kind: Secret\n"),
			},
			wantObjsCount: 2,
		},
		{
			name: "empty object are dropped",
			args: args{
				rawyaml: []byte("---\n" + //empty objects before
					"---\n" +
					"---\n" +
					"apiVersion: v1\n" +
					"kind: ConfigMap\n" +
					"---\n" + // empty objects in the middle
					"---\n" +
					"---\n" +
					"apiVersion: v1\n" +
					"kind: Secret\n" +
					"---\n" + //empty objects after
					"---\n" +
					"---\n"),
			},
			wantObjsCount: 2,
		},
		{
			name: "--- in the middle of objects are ignored",
			args: args{
				[]byte("apiVersion: v1\n" +
					"kind: ConfigMap\n" +
					"data: \n" +
					" key: |\n" +
					"  ··Several lines of text,\n" +
					"  ··with some --- \n" +
					"  ---\n" +
					"  ··in the middle\n" +
					"---\n" +
					"apiVersion: v1\n" +
					"kind: Secret\n"),
			},
			wantObjsCount: 2,
		},
		{
			name: "returns error for invalid yaml",
			args: args{
				rawyaml: []byte("apiVersion: v1\n" +
					"kind: ConfigMap\n" +
					"---\n" +
					"apiVersion: v1\n" +
					"foobar\n" +
					"kind: Secret\n"),
			},
			err: "failed to unmarshal the 2 yaml document",
		},
		{
			name: "returns error for invalid yaml",
			args: args{
				rawyaml: []byte("apiVersion: v1\n" +
					"kind: ConfigMap\n" +
					"---\n" +
					"apiVersion: v1\n" +
					"kind: Pod\n" +
					"---\n" +
					"apiVersion: v1\n" +
					"kind: Deployment\n" +
					"---\n" +
					"apiVersion: v1\n" +
					"foobar\n" +
					"kind: ConfigMap\n"),
			},
			err: "failed to unmarshal the 4 yaml document",
		},
		{
			name: "returns error for invalid yaml",
			args: args{
				rawyaml: []byte("apiVersion: v1\n" +
					"foobar\n" +
					"kind: ConfigMap\n" +
					"---\n" +
					"apiVersion: v1\n" +
					"kind: Secret\n"),
			},
			err: "failed to unmarshal the 1 yaml document",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToUnstructured(bytes.NewReader(tt.args.rawyaml))
			if tt.err != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.err) {
					t.Errorf("expected err %q got %q", tt.err, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected nil error, got %s", err)
				}
				if len(got) != tt.wantObjsCount {
					t.Errorf("expected %d object, got %d", tt.wantObjsCount, len(got))
				}
			}
		})
	}
}
