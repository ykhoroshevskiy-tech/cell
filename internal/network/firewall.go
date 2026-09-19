package network

import (
	"fmt"
	"strings"

	"github.com/ykhoroshevskiy-tech/cell/internal/models"
)

const (
	chainInput   = "CELL_INPUT"
	chainForward = "CELL_FORWARD"
	chainNAT     = "CELL_NAT"
)

var privateCIDRs = []string{
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"127.0.0.0/8",
}

// EnsureFirewall installs global CELL_* iptables chains for the bridge subnet.
func EnsureFirewall() error {
	subnet := fmt.Sprintf("%s/%d", trimHost(models.BridgeHostIP), models.BridgeCIDR)
	if err := ensureChain("filter", chainInput); err != nil {
		return err
	}
	if err := ensureChain("filter", chainForward); err != nil {
		return err
	}
	if err := ensureChain("nat", chainNAT); err != nil {
		return err
	}
	_ = run("iptables", "-D", "INPUT", "-j", chainInput)
	_ = run("iptables", "-D", "FORWARD", "-j", chainForward)
	_ = run("iptables", "-t", "nat", "-D", "POSTROUTING", "-j", chainNAT)

	if err := run("iptables", "-I", "INPUT", "1", "-j", chainInput); err != nil {
		return err
	}
	if err := run("iptables", "-I", "FORWARD", "1", "-j", chainForward); err != nil {
		return err
	}
	if err := run("iptables", "-t", "nat", "-I", "POSTROUTING", "1", "-j", chainNAT); err != nil {
		return err
	}

	if err := flushChain("filter", chainInput); err != nil {
		return err
	}
	if err := flushChain("filter", chainForward); err != nil {
		return err
	}
	if err := flushChain("nat", chainNAT); err != nil {
		return err
	}

	// INPUT: allow established; allow host port 8080; drop guest-sourced traffic to host
	if err := run("iptables", "-A", chainInput, "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT"); err != nil {
		return err
	}
	if err := run("iptables", "-A", chainInput, "-i", models.BridgeName, "-p", "tcp", "--dport", "8080", "-j", "ACCEPT"); err != nil {
		return err
	}
	if err := run("iptables", "-A", chainInput, "-i", models.BridgeName, "-j", "DROP"); err != nil {
		return err
	}

	// FORWARD: established; deny guest->private; deny guest->guest; allow internet egress
	if err := run("iptables", "-A", chainForward, "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT"); err != nil {
		return err
	}
	for _, cidr := range privateCIDRs {
		if err := run("iptables", "-A", chainForward, "-s", subnet, "-d", cidr, "-j", "DROP"); err != nil {
			return err
		}
	}
	if err := run("iptables", "-A", chainForward, "-s", subnet, "-d", subnet, "-j", "DROP"); err != nil {
		return err
	}
	if err := run("iptables", "-A", chainForward, "-s", subnet, "-j", "ACCEPT"); err != nil {
		return err
	}

	// NAT: one subnet-wide MASQUERADE
	if err := run("iptables", "-t", "nat", "-A", chainNAT, "-s", subnet, "-j", "MASQUERADE"); err != nil {
		return err
	}

	_ = writeSysctl("/proc/sys/net/ipv4/ip_forward", "1")
	return nil
}

func chainExists(table, chain string) bool {
	if table == "filter" {
		_, err := output("iptables", "-nL", chain)
		return err == nil
	}
	_, err := output("iptables", "-t", table, "-nL", chain)
	return err == nil
}

func ensureChain(table, chain string) error {
	if chainExists(table, chain) {
		return nil
	}
	args := []string{"-t", table, "-N", chain}
	if table == "filter" {
		args = []string{"-N", chain}
	}
	if err := run("iptables", args...); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return nil
		}
		return err
	}
	return nil
}

func flushChain(table, chain string) error {
	if table == "filter" {
		return run("iptables", "-F", chain)
	}
	return run("iptables", "-t", table, "-F", chain)
}

// FirewallRulesForTest returns rendered iptables args for unit tests.
func FirewallRulesForTest() []string {
	subnet := fmt.Sprintf("%s/%d", trimHost(models.BridgeHostIP), models.BridgeCIDR)
	var rules []string
	rules = append(rules, fmt.Sprintf("-A %s -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT", chainInput))
	rules = append(rules, fmt.Sprintf("-A %s -i %s -p tcp --dport 8080 -j ACCEPT", chainInput, models.BridgeName))
	rules = append(rules, fmt.Sprintf("-A %s -i %s -j DROP", chainInput, models.BridgeName))
	rules = append(rules, fmt.Sprintf("-A %s -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT", chainForward))
	for _, cidr := range privateCIDRs {
		rules = append(rules, fmt.Sprintf("-A %s -s %s -d %s -j DROP", chainForward, subnet, cidr))
	}
	rules = append(rules, fmt.Sprintf("-A %s -s %s -d %s -j DROP", chainForward, subnet, subnet))
	rules = append(rules, fmt.Sprintf("-A %s -s %s -j ACCEPT", chainForward, subnet))
	rules = append(rules, fmt.Sprintf("-t nat -A %s -s %s -j MASQUERADE", chainNAT, subnet))
	return rules
}
