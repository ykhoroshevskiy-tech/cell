package network

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ykhoroshevskiy-tech/cell/internal/models"
	"github.com/ykhoroshevskiy-tech/cell/internal/verbose"
)

type TAPProvider struct{}

func NewTAP() *TAPProvider {
	return &TAPProvider{}
}

func (t *TAPProvider) Setup(cfg *models.NetworkConfig) error {
	verbose.V("net: setup tap=%s guest=%s", cfg.TapName, cfg.GuestIP)
	_ = runQuiet("ip", "tuntap", "del", "dev", cfg.TapName, "mode", "tap")
	if err := run("ip", "tuntap", "add", "dev", cfg.TapName, "mode", "tap"); err != nil {
		return fmt.Errorf("create tap: %w", err)
	}
	_ = runQuiet("ip", "addr", "flush", "dev", cfg.TapName)
	if err := run("ip", "addr", "add", fmt.Sprintf("%s/%d", cfg.HostIP, cfg.CIDR), "dev", cfg.TapName); err != nil {
		return err
	}
	if err := run("ip", "link", "set", cfg.TapName, "up"); err != nil {
		return err
	}
	_ = runQuiet("sysctl", "-w", "net.ipv4.ip_forward=1")

	masqRule := []string{"-t", "nat", "-A", "POSTROUTING", "-s", cfg.GuestIP + "/32", "-j", "MASQUERADE"}
	if err := iptablesEnsure(masqRule); err != nil {
		return err
	}
	return setupFirewall(cfg)
}

func (t *TAPProvider) Teardown(cfg *models.NetworkConfig) error {
	verbose.V("net: teardown tap=%s", cfg.TapName)
	teardownFirewall(cfg)
	_ = iptablesDelete([]string{"-t", "nat", "-D", "POSTROUTING", "-s", cfg.GuestIP + "/32", "-j", "MASQUERADE"})
	_ = runQuiet("ip", "link", "set", cfg.TapName, "down")
	_ = runQuiet("ip", "tuntap", "del", "dev", cfg.TapName, "mode", "tap")
	return nil
}

var privateRanges = []string{
	"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
	"127.0.0.0/8", "169.254.0.0/16",
}

func setupFirewall(cfg *models.NetworkConfig) error {
	gi := cfg.GuestIP
	hi := cfg.HostIP
	for _, rng := range privateRanges {
		rule := []string{"-I", "FORWARD", "1", "-s", gi, "-d", rng, "-j", "DROP"}
		if err := iptablesEnsure(rule); err != nil {
			return err
		}
	}
	rules := [][]string{
		{"-I", "INPUT", "1", "-s", gi, "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT"},
		// Guest OpenCode → infra-proxy on host (default port 8080).
		{"-I", "INPUT", "2", "-s", gi, "-d", hi, "-p", "tcp", "--dport", "8080", "-j", "ACCEPT"},
		{"-I", "INPUT", "3", "-s", gi, "-j", "DROP"},
	}
	for _, rule := range rules {
		if err := iptablesEnsure(rule); err != nil {
			return err
		}
	}
	return nil
}

func teardownFirewall(cfg *models.NetworkConfig) {
	gi := cfg.GuestIP
	hi := cfg.HostIP
	for _, rng := range privateRanges {
		_ = iptablesDelete([]string{"-D", "FORWARD", "-s", gi, "-d", rng, "-j", "DROP"})
	}
	_ = iptablesDelete([]string{"-D", "INPUT", "-s", gi, "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT"})
	_ = iptablesDelete([]string{"-D", "INPUT", "-s", gi, "-d", hi, "-p", "tcp", "--dport", "8080", "-j", "ACCEPT"})
	_ = iptablesDelete([]string{"-D", "INPUT", "-s", gi, "-j", "DROP"})
}

func iptablesEnsure(insertRule []string) error {
	checkRule := append([]string{"-C"}, insertRule[1:]...)
	if insertRule[0] == "-I" {
		checkRule = append([]string{"-C"}, insertRule[2:]...)
	}
	if runQuiet("iptables", checkRule...) == nil {
		return nil
	}
	return run("iptables", insertRule...)
}

func iptablesDelete(rule []string) error {
	return runQuiet("iptables", rule...)
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if verbose.Enabled() {
		verbose.V("net: %s %s", name, strings.Join(args, " "))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, out)
	}
	return nil
}

func runQuiet(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}
