// Copyright © 2015-2017 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

package bgp

import (
	"testing"
	"time"

	"github.com/platinasystems/go/internal/test"
	"github.com/platinasystems/go/internal/test/docker"
)

type docket struct {
	docker.Docket
}

var Suite = test.Suite{
	Name: "bgp",
	Tests: test.Tests{
		&docket{
			docker.Docket{
				Name: "eth",
				Tmpl: "testdata/bird/bgp/conf.yaml.tmpl",
			},
		},
		&docket{
			docker.Docket{
				Name: "vlan",
				Tmpl: "testdata/bird/bgp/vlan/conf.yaml.tmpl",
			},
		},
	},
}

func (d *docket) Run(t *testing.T) {
	d.UTS(t, []test.UnitTest{
		test.UnitTest{"connectivity", d.checkConnectivity},
		test.UnitTest{"bird", d.checkBird},
		test.UnitTest{"neighbors", d.checkNeighbors},
		test.UnitTest{"routes", d.checkRoutes},
		test.UnitTest{"inter-connectivity", d.checkInterConnectivity},
		test.UnitTest{"flap", d.checkFlap},
	})
}

func (d *docket) checkConnectivity(t *testing.T) {
	assert := test.Assert{t}

	for _, x := range []struct {
		host   string
		target string
	}{
		{"R1", "192.168.120.10"},
		{"R1", "192.168.150.4"},
		{"R2", "192.168.222.2"},
		{"R2", "192.168.120.5"},
		{"R3", "192.168.222.10"},
		{"R3", "192.168.111.4"},
		{"R4", "192.168.111.2"},
		{"R4", "192.168.150.5"},
	} {
		out, err := d.ExecCmd(t, x.host, "ping", "-c3", x.target)
		assert.Nil(err)
		assert.Match(out, "[1-3] packets received")
	}
}

func (d *docket) checkBird(t *testing.T) {
	assert := test.Assert{t}
	time.Sleep(1 * time.Second)
	for _, r := range d.Config.Routers {
		t.Logf("Checking BIRD on %v", r.Hostname)
		out, err := d.ExecCmd(t, r.Hostname, "ps", "ax")
		assert.Nil(err)
		assert.Match(out, ".*bird.*")
	}
}

func (d *docket) checkNeighbors(t *testing.T) {
	assert := test.Assert{t}

	for _, x := range []struct {
		hostname string
		peer     string
	}{
		{"R1", "R2"},
		{"R1", "R4"},
		{"R2", "R1"},
		{"R2", "R3"},
		{"R3", "R2"},
		{"R3", "R4"},
		{"R4", "R1"},
		{"R4", "R3"},
	} {
		found := false
		timeout := 120

		for i := timeout; i > 0; i-- {
			out, err := d.ExecCmd(t, x.hostname,
				"birdc", "show", "protocols", "all", x.peer)
			assert.Nil(err)
			if !assert.MatchNonFatal(out, ".*Established.*") {
				time.Sleep(1 * time.Second)
			} else {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("No bgp peer established for %v", x.hostname)
		}
	}
}

func (d *docket) checkRoutes(t *testing.T) {
	assert := test.Assert{t}

	for _, x := range []struct {
		hostname string
		route    string
	}{
		{"R1", "192.168.222.0/24"},
		{"R1", "192.168.111.0/24"},
		{"R2", "192.168.150.0/24"},
		{"R2", "192.168.111.0/24"},
		{"R3", "192.168.120.0/24"},
		{"R3", "192.168.150.0/24"},
		{"R4", "192.168.120.0/24"},
		{"R4", "192.168.222.0/24"},
	} {
		found := false
		timeout := 60
		for i := timeout; i > 0; i-- {
			out, err := d.ExecCmd(t, x.hostname,
				"ip", "route", "show", x.route)
			assert.Nil(err)
			if !assert.MatchNonFatal(out, x.route) {
				time.Sleep(1 * time.Second)
			} else {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("No bgp route for %v: %v", x.hostname, x.route)
		}
	}
}

func (d *docket) checkInterConnectivity(t *testing.T) {
	assert := test.Assert{t}

	for _, x := range []struct {
		hostname string
		target   string
	}{
		{"R1", "192.168.222.2"},
		{"R1", "192.168.111.2"},
		{"R2", "192.168.111.4"},
		{"R2", "192.168.150.4"},
		{"R3", "192.168.120.5"},
		{"R3", "192.168.150.5"},
		{"R4", "192.168.120.10"},
		{"R4", "192.168.222.10"},
	} {
		out, err := d.ExecCmd(t, x.hostname,
			"ping", "-c3", x.target)
		assert.Nil(err)
		assert.Match(out, "[1-3] packets received")
		assert.Program(test.Self{}, "vnet", "show", "ip", "fib")
	}
}

func (d *docket) checkFlap(t *testing.T) {
	assert := test.Assert{t}

	for _, r := range d.Config.Routers {
		for _, i := range r.Intfs {
			var intf string
			if i.Vlan != "" {
				intf = i.Name + "." + i.Vlan
			} else {
				intf = i.Name
			}
			_, err := d.ExecCmd(t, r.Hostname,
				"ip", "link", "set", "down", intf)
			assert.Nil(err)
			time.Sleep(1 * time.Second)
			_, err = d.ExecCmd(t, r.Hostname,
				"ip", "link", "set", "up", intf)
			assert.Nil(err)
			time.Sleep(1 * time.Second)
			assert.Program(test.Self{}, "vnet", "show", "ip", "fib")
		}
	}
}
