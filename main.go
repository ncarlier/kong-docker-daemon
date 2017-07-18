// Copyright © 2017 Nicolas Carlier <n.carlier@nunux.org>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"flag"
	"io"
	"os"
	"reflect"
	"regexp"

	"net/http"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
	"github.com/ncarlier/kong-docker-daemon/pkg/kong"
	"github.com/ncarlier/kong-docker-daemon/pkg/toolkit"
	"github.com/sirupsen/logrus"
)

// Version App version
var Version = "snapshot"

// UpstreamConfig upstream configuration structure
type UpstreamConfig map[string][]string

// KongUpstreamLabel Kong upstream label
const KongUpstreamLabel = "kong.upstream"

// KongClient Kong API client
var KongClient *kong.Client

// DockerClient Docker API client
var DockerClient *client.Client

var portRE = regexp.MustCompile("^[0-9]+")

var (
	kongAdminURL string
	debug        bool
	verbose      bool
)

func init() {
	logLevel := "warn"
	if val, ok := os.LookupEnv("APP_LOG_LEVEL"); ok {
		logLevel = val
	}
	defaultKongURL := "http://localhost:8001"
	if val, ok := os.LookupEnv("KONG_ADMIN_URL"); ok {
		defaultKongURL = val
	}
	const usage = "Kong admin API URL"

	flag.StringVar(&kongAdminURL, "kong-admin-url", defaultKongURL, usage)
	flag.StringVar(&kongAdminURL, "k", defaultKongURL, usage+" (shorthand)")
	flag.BoolVar(&verbose, "v", false, "verbose output")
	flag.BoolVar(&debug, "d", false, "debug output")

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	logrus.SetOutput(os.Stdout)
	// Only log the debug severity or above
	lvl, err := logrus.ParseLevel(logLevel)
	if err != nil {
		lvl = logrus.WarnLevel
	}
	logrus.SetLevel(lvl)
}

func main() {
	// Parse CLI flags
	flag.Parse()

	// Adjust logger level
	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	} else if verbose {
		logrus.SetLevel(logrus.InfoLevel)
	}

	// Init. Docker client
	var err error
	DockerClient, err = client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	// Validate Docker connection...
	ctx := context.Background()
	dockerVersion, err := DockerClient.ServerVersion(ctx)
	if err != nil {
		panic(err)
	}
	logrus.Debugf("connection established with Docker %s (API %s)", dockerVersion.Version, dockerVersion.APIVersion)

	// Init. Kong client
	KongClient = kong.NewKongClient(http.DefaultClient, kongAdminURL)

	// Validate Kong connection...
	kongInfos, err := KongClient.GetNodeInformation()
	if err != nil {
		panic(err)
	}
	logrus.Debugf("connection established with Kong %s", kongInfos.Version)

	// Synchronize upstreams at startup...
	if err = synchronizeUpstreams(); err != nil {
		panic(err)
	}

	// Listening Docker events...
	events, errors := DockerClient.Events(ctx, types.EventsOptions{})
	for {
		select {
		case err := <-errors:
			if err != nil && err != io.EOF {
				panic(err)
			}
			break
		case evt := <-events:
			if shouldProcessEvent(evt) {
				upstream := evt.Actor.Attributes[KongUpstreamLabel]
				if err = synchronizeUpstreams(); err != nil {
					logrus.WithError(err).WithField("upstream", upstream).Error("upstream synchronization error")
				} else if evt.Action == "die" {
					cleanupOrphanUpstream(upstream)
				}
			}
		}
	}
}

func getUpstreamConfigurationFromDocker(upstream string) (*UpstreamConfig, error) {
	// Init config
	config := make(UpstreamConfig)

	// Get Containers
	options := types.ContainerListOptions{}
	containers, err := DockerClient.ContainerList(context.Background(), options)
	if err != nil {
		return nil, err
	}

	// Iterate over running containers
	for _, c := range containers {
		running := c.State == "running"
		if ups, ok := c.Labels[KongUpstreamLabel]; ok && running {
			if upstream != "" && upstream != ups {
				continue
			}
			target, err := getContainerTarget(c.ID)
			if err != nil || target == "" {
				continue
			}
			// Add container target to the configuration
			config[ups] = append(config[ups], target)
		}
	}
	return &config, nil
}

func getUpstreamConfigurationFromKong(upstream string) (*UpstreamConfig, error) {
	// Init config
	config := make(UpstreamConfig)
	// Retrieve upstream from Kong...
	ups, err := KongClient.RetrieveUpstream(upstream)
	if err != nil {
		return nil, err
	}
	// Creat upstream if not found...
	if ups == nil {
		logrus.WithFields(logrus.Fields{
			"upstream": upstream,
		}).Debug("upstream not found in Kong: creating...")
		if err := KongClient.UpdateOrCreateUpstream(upstream); err != nil {
			return nil, err
		}
		logrus.WithFields(logrus.Fields{
			"upstream": upstream,
		}).Info("upstream created in Kong")
	}
	// Retrieve current upstream targets...
	targets, err := KongClient.ListActiveTargets(upstream)
	if err != nil {
		return nil, err
	}
	for _, target := range targets.Data {
		config[upstream] = append(config[upstream], target.Target)
	}
	return &config, nil
}

func synchronizeUpstreams() error {
	// Get upstream configuration for ALL containers
	dockerUpstreams, err := getUpstreamConfigurationFromDocker("")
	if err != nil {
		return err
	}
	logrus.WithFields(logrus.Fields{
		"config": *dockerUpstreams,
	}).Debug("docker upstream configuration")
	for upstream, dockerTargets := range *dockerUpstreams {
		// Get Kong upstream configuration
		kongUpstreams, err := getUpstreamConfigurationFromKong(upstream)
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"upstream": upstream,
			}).Error("unable synchronize upstream configurations")
			continue
		}
		logrus.WithFields(logrus.Fields{
			"config": *kongUpstreams,
		}).Debug("kong upstream configuration")
		kongTargets := (*kongUpstreams)[upstream]
		targetsToRemove := toolkit.Diff(kongTargets, dockerTargets)
		targetsToCreate := toolkit.Diff(dockerTargets, kongTargets)
		// Unregister deprecated targets
		if err = unRegisterUpstreamTargets(upstream, targetsToRemove); err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"upstream": upstream,
				"targets":  targetsToRemove,
			}).Error("unable to unregister upstream targets from Kong")
			continue
		}
		// Register new targets
		if err = registerUpstreamTargets(upstream, targetsToCreate); err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"upstream": upstream,
				"targets":  targetsToCreate,
			}).Error("unable to register new upstream targets")
			continue
		}
		logrus.WithFields(logrus.Fields{
			"upstream": upstream,
			"created":  targetsToCreate,
			"removed":  targetsToRemove,
		}).Info("upstream configuration synchronized")
	}
	return nil
}

func registerUpstreamTargets(upstream string, targets []string) error {
	logrus.WithFields(logrus.Fields{
		"upstream": upstream,
		"targets":  targets,
	}).Debug("registering upstream targets...")
	for _, target := range targets {
		tgt, err := KongClient.AddTarget(upstream, target, 100)
		if err != nil {
			return err
		}
		logrus.WithFields(logrus.Fields{
			"id":          tgt.ID,
			"target":      tgt.Target,
			"upstream":    upstream,
			"upstream_id": tgt.UpstreamID,
		}).Debug("upstream registered")
	}
	return nil
}

func unRegisterUpstreamTargets(upstream string, targets []string) error {
	logrus.WithFields(logrus.Fields{
		"upstream": upstream,
		"targets":  targets,
	}).Debug("un-registering upstream targets...")
	activeTargets, err := KongClient.ListActiveTargets(upstream)
	if err != nil {
		return err
	}
	for _, target := range targets {
		for _, tgt := range activeTargets.Data {
			if target == tgt.Target {
				err := KongClient.DeleteTarget(upstream, tgt.ID)
				if err != nil {
					return err
				}
			}
		}
		logrus.WithFields(logrus.Fields{
			"upstream": upstream,
			"target":   target,
		}).Debug("upstream un-registered")
	}
	return nil
}

func shouldProcessEvent(evt events.Message) bool {
	// fmt.Printf("EVENT: %+v\n", evt)
	action := evt.Action
	if evt.Type == "container" && (action == "start" || action == "die") {
		if _, ok := evt.Actor.Attributes[KongUpstreamLabel]; ok {
			return ok
		}
	}
	return false
}

func getContainerTarget(ID string) (string, error) {
	// Get container target
	container, err := DockerClient.ContainerInspect(context.Background(), ID)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"container": ID,
		}).Error("unable to get container details")
		return "", err
	}
	// Resolve container port...
	ports := reflect.ValueOf(container.Config.ExposedPorts).MapKeys()
	if len(ports) == 0 {
		logrus.WithFields(logrus.Fields{
			"container": container.ID,
		}).Warn("no port exposed -> container ignored")
		return "", nil
	}
	port := portRE.FindString(ports[0].String())
	// Resolve container IP...
	var ip string
	if container.NetworkSettings.IPAddress != "" {
		ip = container.NetworkSettings.IPAddress
	} else if len(container.NetworkSettings.Networks) > 0 {
		for _, network := range container.NetworkSettings.Networks {
			ip = network.IPAddress
			break
		}
	} else if "host" == container.HostConfig.NetworkMode {
		ip = "127.0.0.1"
	} else {
		// TODO get host IP
	}

	target := ip + ":" + port
	logrus.WithFields(logrus.Fields{
		"container": container.ID,
		"name":      container.Name,
		"target":    target,
	}).Debug("resolved container target")
	return target, nil
}

func cleanupOrphanUpstream(upstream string) error {
	dockerUpstreams, err := getUpstreamConfigurationFromDocker(upstream)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"upstream": upstream,
		}).Error("unable to clean orphan upstream")
		return err
	}
	if (*dockerUpstreams)[upstream] != nil {
		// The upstream is not an orphan. Abort.
		return nil
	}
	result, err := KongClient.ListActiveTargets(upstream)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"upstream": upstream,
		}).Error("unable to clean orphan upstream")
		return err
	}
	if result.Total == 1 {
		target := result.Data[0].Target
		logrus.WithFields(logrus.Fields{
			"upstream": upstream,
			"removing": target,
		}).Info("upstream is an orphan: cleaning...")
		err = KongClient.DeleteTarget(upstream, result.Data[0].ID)
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"upstream": upstream,
				"removing": target,
			}).Error("unable to clean orphan upstream")
			return err
		}
		logrus.WithFields(logrus.Fields{
			"upstream": upstream,
			"removed":  target,
		}).Info("upstream was an orphan: cleaned")
	}
	return nil
}
