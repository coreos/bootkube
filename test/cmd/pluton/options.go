// Copyright 2015 CoreOS, Inc.
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

import "github.com/coreos/mantle/pluton/harness"

func init() {
	root.AddCommand(cmdRun)
	root.AddCommand(cmdList)

	sv := root.PersistentFlags().StringVar
	bv := root.PersistentFlags().BoolVar

	// general options
	sv(&harness.Opts.OutputDir, "output-dir", "_pluton_temp", "Temporary output directory for test data and logs")
	sv(&harness.Opts.CloudPlatform, "platform", "gce", "VM platform: qemu, gce, aws")
	root.PersistentFlags().IntVar(&harness.Opts.Parallel, "parallel", 1, "number of tests to run in parallel")
	sv(&harness.Opts.PlatformOptions.BaseName, "basename", "pluton", "Cluster name prefix")
	sv(&harness.Opts.BootkubeRepo, "bootkubeRepo", "quay.io/coreos/bootkube", "")
	sv(&harness.Opts.BootkubeTag, "bootkubeTag", "v0.3.11", "")
	sv(&harness.Opts.BootkubeScriptDir, "bootkubeScriptDir", "", "Make use of bootkube's node init scripts and kubelet service files. Leave blank to use default or pass in the hack/quickstart dir from the bootkube repo.")

	// gce-specific options
	sv(&harness.Opts.GCEOptions.Image, "gce-image", "projects/coreos-cloud/global/images/coreos-stable-1298-6-0-v20170315", "GCE image")
	sv(&harness.Opts.GCEOptions.Project, "gce-project", "coreos-gce-testing", "GCE project name")
	sv(&harness.Opts.GCEOptions.Zone, "gce-zone", "us-central1-a", "GCE zone name")
	sv(&harness.Opts.GCEOptions.MachineType, "gce-machinetype", "n1-standard-1", "GCE machine type")
	sv(&harness.Opts.GCEOptions.DiskType, "gce-disktype", "pd-ssd", "GCE disk type")
	sv(&harness.Opts.GCEOptions.Network, "gce-network", "default", "GCE network")
	bv(&harness.Opts.GCEOptions.ServiceAuth, "gce-service-auth", false, "for non-interactive auth when running within GCE")

	// future choice
	harness.Opts.CloudPlatform = "gce"

}
