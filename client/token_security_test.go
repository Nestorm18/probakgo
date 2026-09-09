package main

import (
	"errors"
	"reflect"
	"testing"
)

func TestMonitoringTokenUsesSeparatedReadOnlyPrivileges(t *testing.T) {
	var commands [][]string
	err := restrictPVEToken("probakgo-client", func(args ...string) error { commands = append(commands, args); return nil })
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"user", "token", "modify", "root@pam", "probakgo-client", "--privsep", "1"},
		{"acl", "modify", "/", "--tokens", "root@pam!probakgo-client", "--roles", "PVEAuditor", "--propagate", "1"},
	}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("unsafe PVE token policy: %v", commands)
	}
	if err := restrictPVEToken("probakgo-client", func(...string) error { return errors.New("permission denied") }); err == nil {
		t.Fatal("token restriction failure was ignored")
	}
}
