// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package api

import "testing"

func TestValidateObjectsListPrefix(t *testing.T) {
	t.Parallel()
	ok := []string{"", "builds", "builds/", "builds/image.tar"}
	for _, prefix := range ok {
		if err := validateObjectsListPrefix(prefix); err != nil {
			t.Fatalf("prefix %q: %v", prefix, err)
		}
	}
	bad := []string{"/abs", "a/../b", ".."}
	for _, prefix := range bad {
		if err := validateObjectsListPrefix(prefix); err == nil {
			t.Fatalf("prefix %q: expected error", prefix)
		}
	}
}

func TestParseObjectsListPageSize(t *testing.T) {
	t.Parallel()
	n, err := parseObjectsListPageSize("")
	if err != nil || n != objectsListDefaultPageSize {
		t.Fatalf("default: n=%d err=%v", n, err)
	}
	n, err = parseObjectsListPageSize(" 10 ")
	if err != nil || n != 10 {
		t.Fatalf("10: n=%d err=%v", n, err)
	}
	if _, err := parseObjectsListPageSize("1000"); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"0", "-1", "1001", "abc"} {
		if _, err := parseObjectsListPageSize(raw); err == nil {
			t.Fatalf("pageSize %q: expected error", raw)
		}
	}
}
