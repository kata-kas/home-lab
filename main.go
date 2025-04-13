package main

import (
	"github.com/pulumi/pulumi-command/sdk/go/command/remote"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		serverPublicIp := cfg.Require("serverPublicIp")
		userName := cfg.Require("userName")

		conn := remote.ConnectionArgs{
			Host: pulumi.String(serverPublicIp),
			User: pulumi.String(userName),
		}
		checkK3s, err := remote.NewCommand(ctx, "checkK3s", &remote.CommandArgs{
			Connection: conn,
			Create:     pulumi.String("if systemctl is-active --quiet k3s; then echo 'already_installed'; else echo 'not_installed'; fi"),
		})
		if err != nil {
			return err
		}
		cleanupK3s, err := remote.NewCommand(ctx, "cleanupK3s", &remote.CommandArgs{
			Connection: conn,
			Create:     pulumi.String("if [ -f /usr/local/bin/k3s-uninstall.sh ]; then sudo /usr/local/bin/k3s-uninstall.sh; echo 'cleaned'; else echo 'no_cleanup_needed'; fi"),
		}, pulumi.DependsOn([]pulumi.Resource{checkK3s}))
		if err != nil {
			return err
		}
		installK3s, err := remote.NewCommand(ctx, "installK3s", &remote.CommandArgs{
			Connection: conn,
			Create:     pulumi.String("curl -sfL https://get.k3s.io | sudo sh -s - --write-kubeconfig-mode 644"),
		}, pulumi.DependsOn([]pulumi.Resource{cleanupK3s}))
		if err != nil {
			return err
		}
		verifyK3s, err := remote.NewCommand(ctx, "verifyK3s", &remote.CommandArgs{
			Connection: conn,
			Create:     pulumi.String("systemctl is-active --quiet k3s && echo 'K3s is running' || echo 'K3s installation failed'"),
		}, pulumi.DependsOn([]pulumi.Resource{installK3s}))
		if err != nil {
			return err
		}
		setupKubectl, err := remote.NewCommand(ctx, "setupKubectl", &remote.CommandArgs{
			Connection: conn,
			Create:     pulumi.String("mkdir -p $HOME/.kube && sudo cp /etc/rancher/k3s/k3s.yaml $HOME/.kube/config && sudo chown $(id -u):$(id -g) $HOME/.kube/config && echo 'kubectl configured'"),
		}, pulumi.DependsOn([]pulumi.Resource{verifyK3s}))
		if err != nil {
			return err
		}

		ctx.Export("k3sStatus", verifyK3s.Stdout)
		ctx.Export("kubectlStatus", setupKubectl.Stdout)
		return nil
	})
}
