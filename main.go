package main

import (
	"context"
	"flag"

	"github.com/ahmet2mir/terraform-provider-calico/calico"
	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
	"k8s.io/klog"
)

func main() {
	debugFlag := flag.Bool("debug", false, "Start provider in stand-alone debug mode.")
	flag.Parse()
	klogFlags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(klogFlags)
	err := klogFlags.Set("logtostderr", "false")
	if err != nil {
		panic(err)
	}
	serveOpts := &plugin.ServeOpts{
		ProviderFunc: calico.Provider,
	}
	if debugFlag != nil && *debugFlag {
		plugin.Debug(context.Background(), "registry.terraform.io/ahmet2mir/calico", serveOpts)
	} else {
		plugin.Serve(serveOpts)
	}
}
