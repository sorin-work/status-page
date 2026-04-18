package store

import (
	"context"
	"errors"
	"testing"
)

func TestComponents(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	web, err := s.CreateComponent(ctx, Component{Name: " Website ", Description: "Marketing site"})
	if err != nil {
		t.Fatal(err)
	}
	api, err := s.CreateComponent(ctx, Component{Name: "API"})
	if err != nil {
		t.Fatal(err)
	}
	if web.Name != "Website" || web.Position != 1 || api.Position != 2 {
		t.Errorf("created: %+v %+v", web, api)
	}

	api.Position = 0
	if err := s.UpdateComponent(ctx, api); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListComponents(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Name != "API" {
		t.Errorf("order after moving API up: %+v", list)
	}

	got, err := s.GetComponent(ctx, web.ID)
	if err != nil || got.Description != "Marketing site" {
		t.Errorf("get: %+v %v", got, err)
	}
	m := sampleMonitor("homepage")
	m.ComponentID = web.ID
	created, err := s.CreateMonitor(ctx, m)
	if err != nil {
		t.Fatal(err)
	}
	members, err := s.ListComponentMonitors(ctx, web.ID)
	if err != nil || len(members) != 1 || members[0].ComponentID != web.ID {
		t.Fatalf("component monitors: %+v %v", members, err)
	}

	if err := s.DeleteComponent(ctx, web.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetComponent(ctx, web.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleted component still found: %v", err)
	}
	if detached, _ := s.GetMonitor(ctx, created.ID); detached.ComponentID != 0 {
		t.Errorf("monitor still points at deleted component: %+v", detached)
	}
	if err := s.UpdateComponent(ctx, Component{ID: 99, Name: "x"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("update missing: %v", err)
	}
}
