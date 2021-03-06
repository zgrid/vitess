// Copyright 2016, Google Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package gateway contains the routing layer of vtgate. A Gateway can take
// a query targeted to a keyspace/shard/tablet_type and send it off.
package gateway

import (
	"flag"
	"time"

	log "github.com/golang/glog"
	"golang.org/x/net/context"

	"github.com/youtube/vitess/go/vt/discovery"
	"github.com/youtube/vitess/go/vt/tabletserver/tabletconn"
	"github.com/youtube/vitess/go/vt/topo"

	topodatapb "github.com/youtube/vitess/go/vt/proto/topodata"
)

// This file contains the Gateway interface definition, and the
// implementations registry.

var (
	implementation       = flag.String("gateway_implementation", "discoverygateway", "The implementation of gateway")
	initialTabletTimeout = flag.Duration("gateway_initial_tablet_timeout", 30*time.Second, "At startup, the gateway will wait up to that duration to get one tablet per keyspace/shard/tablettype")
)

// A Gateway is the query processing module for each shard,
// which is used by ScatterConn.
type Gateway interface {
	tabletconn.TabletConn

	// WaitForTablets asks the gateway to wait for the provided
	// tablets types to be available. It the context is canceled
	// before the end, it should return ctx.Err().
	WaitForTablets(ctx context.Context, tabletTypesToWait []topodatapb.TabletType) error

	// CacheStatus returns a list of TabletCacheStatus per tablet.
	CacheStatus() TabletCacheStatusList
}

// Creator is the factory method which can create the actual gateway object.
type Creator func(hc discovery.HealthCheck, topoServer topo.Server, serv topo.SrvTopoServer, cell string, retryCount int) Gateway

var creators = make(map[string]Creator)

// RegisterCreator registers a Creator with given name.
func RegisterCreator(name string, gc Creator) {
	if _, ok := creators[name]; ok {
		log.Fatalf("Gateway %s already exists", name)
	}
	creators[name] = gc
}

// GetCreator returns the Creator specified by the gateway_implementation flag.
func GetCreator() Creator {
	gc, ok := creators[*implementation]
	if !ok {
		log.Fatalf("No gateway registered as %s", *implementation)
	}
	return gc
}

// WaitForTablets is a helper method to wait for the provided tablets,
// up until the *initialTabletTimeout. It will log what it is doing.
// Note it has the same name as the Gateway's interface method, as it
// just calls it.
func WaitForTablets(gw Gateway, tabletTypesToWait []topodatapb.TabletType) error {
	log.Infof("Gateway waiting for serving tablets...")
	ctx, cancel := context.WithTimeout(context.Background(), *initialTabletTimeout)
	defer cancel()

	err := gw.WaitForTablets(ctx, tabletTypesToWait)
	switch err {
	case nil:
		log.Infof("Waiting for tablets completed")
		// all good
	case context.DeadlineExceeded:
		log.Warningf("Timeout waiting for all keyspaces / shards to have healthy tablets, may be in degraded mode")
	default:
		log.Errorf("gateway.WaitForTablets failed: %v", err)
	}
	return err
}
