// Copyright 2018 Sergiusz Bazanski
//
// Permission to use, copy, modify, and/or distribute this software for any
// purpose with or without fee is hereby granted.
//
// THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
// WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
// MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY
// SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
// WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN ACTION
// OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF OR IN
// CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.

package main

import (
	"context"
	"encoding/xml"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/golang/glog"
)

var (
	flagRegister string
	flagOutput   string
	flagRedirect string
)

type registry struct {
	XMLName xml.Name        `xml:"Rejestr"`
	Entries []registryEntry `xml:"PozycjaRejestru"`
}

type registryEntry struct {
	Address string `xml:"AdresDomeny"`
}

func getList(ctx context.Context) (registry, error) {
	registry := registry{}

	resp, err := http.Get(flagRegister)
	if err != nil {
		return registry, fmt.Errorf("while connecting to registry: %v", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return registry, fmt.Errorf("while downloading registry: %v", err)
	}

	err = xml.Unmarshal([]byte(body), &registry)
	if err != nil {
		return registry, fmt.Errorf("while parsing registry: %v", err)
	}

	if len(registry.Entries) == 0 {
		return registry, fmt.Errorf("zero results in registry")
	}

	return registry, nil
}

func populateConfig(ctx context.Context) error {
	entries := []registryEntry{}
	operation := func() error {
		glog.Info("Trying to download registry...")
		registry, err := getList(ctx)
		if err != nil {
			glog.Errorf("Could not get registry: %v", err)
			return err
		}
		entries = registry.Entries
		return nil
	}
	err := backoff.Retry(operation, backoff.NewExponentialBackOff())
	if err != nil {
		return fmt.Errorf("while populating config: %v", err)
	}

	glog.Infof("Got registry: %d entries", len(entries))

	domainSet := make(map[string]bool)
	domains := make([]string, 0, len(entries))
	for _, entry := range entries {
		addr := entry.Address
		if domainSet[addr] {
			continue
		}
		domainSet[addr] = true
		domains = append(domains, addr)
	}
	sort.Slice(domains, func(i, j int) bool { return domains[i] < domains[j] })

	glog.Infof("After deduplication: %d entries", len(domains))

	configLines := make([]string, len(domains)*2)
	for i, domain := range domains {
		configLines[i*2] = fmt.Sprintf("local-zone: %q redirect", domain)
		configLines[i*2+1] = fmt.Sprintf("local-data: \"%s A %s\"", domain, flagRedirect)
	}

	config := strings.Join(configLines, "\n")
	operation = func() error {
		glog.Info("Trying to write config...")
		err := ioutil.WriteFile(flagOutput, []byte(config), 0644)
		if err != nil {
			glog.Errorf("Could not write config: %v", err)
			return err
		}
		return nil
	}
	err = backoff.Retry(operation, backoff.NewExponentialBackOff())
	if err != nil {
		return fmt.Errorf("while populating config: %v", err)
	}

	return nil
}

func reloadUnbound(ctx context.Context) error {
	//TODO(q3k): Use unbound remote-control?
	cmd := exec.CommandContext(ctx, "systemctl", "reload", "unbound")
	err := cmd.Run()
	return err
}

func run(ctx context.Context) {
	ticker := time.NewTicker(time.Hour * 6)
	for {
		select {
		case <-ctx.Done():
			glog.Info("Stoppping runner.")
			return
		case <-ticker.C:
			glog.Info("Starting periodic config update...")
			err := populateConfig(ctx)
			if err != nil {
				glog.Error(err)
				continue
			}
			glog.Info("Periodic config update done. Reloading unbound.")
			reloadUnbound(ctx)
			glog.Info("Unbound reloaded.")
		}
	}
}

func main() {
	flag.StringVar(&flagRegister, "register_endpoint", "https://www.hazard.mf.gov.pl/api/Register", "Address of RSH Registry endpoint")
	flag.StringVar(&flagOutput, "output", "/etc/unbound/rsh.conf", "Path to generated Unbound config file")
	flag.StringVar(&flagRedirect, "redirect", "145.237.235.240", "Address to redirect to")
	flag.Parse()
	glog.Info("Starting up...")

	ctx := context.Background()

	err := populateConfig(ctx)
	if err != nil {
		glog.Errorf("While populating config at startup: %v", err)
		glog.Error("Continuing anyway...")
	} else {
		err = reloadUnbound(ctx)
		if err != nil {
			glog.Errorf("While reloading unbound at startup: %v", err)
			glog.Error("Continuing anyway...")
		}
	}
	glog.Info("Initial config update done.")
	go run(ctx)
	select {}
}
