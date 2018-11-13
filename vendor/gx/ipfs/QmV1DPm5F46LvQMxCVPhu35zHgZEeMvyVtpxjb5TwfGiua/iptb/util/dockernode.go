package iptbutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gx/ipfs/QmZLUtHGe9HDQrreAYkXCzzK6mHVByV4MRd8heXAtV5wyS/stump"
	cnet "gx/ipfs/QmfEZa44SyWfyXpkbVfi19H1QpY73DU6E5omK2HbKXwqR6/go-ctrlnet"
)

// DockerNode is an IPFS node in a docker container controlled
// by IPTB
type DockerNode struct {
	ImageName string
	ID        string

	apiAddr string

	LocalNode
}

// assert DockerNode satisfies the testbed IpfsNode interface
var _ IpfsNode = (*DockerNode)(nil)

func (dn *DockerNode) Start(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("cannot yet pass daemon args to docker nodes")
	}

	cmd := exec.Command("docker", "run", "-d", "-v", dn.Dir+":/data/ipfs", dn.ImageName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, string(out))
	}

	id := bytes.TrimSpace(out)
	dn.ID = string(id)
	idfile := filepath.Join(dn.Dir, "dockerID")
	err = ioutil.WriteFile(idfile, id, 0664)

	if err != nil {
		killErr := dn.killContainer()
		if killErr != nil {
			return combineErrors(err, killErr)
		}
		return err
	}

	err = waitOnAPI(dn)
	if err != nil {
		killErr := dn.Kill()
		if killErr != nil {
			return combineErrors(err, killErr)
		}
		return err
	}

	return nil
}

func combineErrors(err1, err2 error) error {
	return fmt.Errorf("%v\nwhile handling the above error, the following error occurred:\n%v", err1, err2)
}

func (dn *DockerNode) setAPIAddr() error {
	internal, err := dn.LocalNode.APIAddr()
	if err != nil {
		return err
	}

	port := strings.Split(internal, ":")[1]

	dip, err := dn.getDockerIP()
	if err != nil {
		return err
	}

	dn.apiAddr = dip + ":" + port

	maddr := []byte("/ip4/" + dip + "/tcp/" + port)
	return ioutil.WriteFile(filepath.Join(dn.Dir, "api"), maddr, 0644)
}

func (dn *DockerNode) APIAddr() (string, error) {
	if dn.apiAddr == "" {
		if err := dn.setAPIAddr(); err != nil {
			return "", err
		}
	}

	return dn.apiAddr, nil
}

func (dn *DockerNode) getDockerIP() (string, error) {
	cmd := exec.Command("docker", "inspect", dn.ID)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %s", err, string(out))
	}

	var info []interface{}
	if err := json.Unmarshal(out, &info); err != nil {
		return "", err
	}

	if len(info) == 0 {
		return "", fmt.Errorf("got no inspect data")
	}

	cinfo := info[0].(map[string]interface{})
	netinfo := cinfo["NetworkSettings"].(map[string]interface{})
	return netinfo["IPAddress"].(string), nil
}

func (dn *DockerNode) Kill() error {
	err := dn.killContainer()
	if err != nil {
		return err
	}
	return os.Remove(filepath.Join(dn.Dir, "dockerID"))
}

func (dn *DockerNode) killContainer() error {
	out, err := exec.Command("docker", "kill", "--signal=INT", dn.ID).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, string(out))
	}
	return nil
}

func (dn *DockerNode) String() string {
	return "docker:" + dn.PeerID
}

func (dn *DockerNode) RunCmd(args ...string) (string, error) {
	if dn.ID == "" {
		return "", fmt.Errorf("no docker id set on node")
	}

	args = append([]string{"exec", "-ti", dn.ID}, args...)
	cmd := exec.Command("docker", args...)
	cmd.Stdin = os.Stdin

	stump.VLog("running: ", cmd.Args)

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s: %s", err, string(out))
	}

	return string(out), nil
}

func (dn *DockerNode) Shell() error {
	nodes, err := LoadNodes()
	if err != nil {
		return err
	}

	nenvs := os.Environ()
	for i, n := range nodes {
		peerid := n.GetPeerID()
		if peerid == "" {
			return fmt.Errorf("failed to check peerID")
		}

		nenvs = append(nenvs, fmt.Sprintf("NODE%d=%s", i, peerid))
	}

	cmd := exec.Command("docker", "exec", "-ti", dn.ID, "/bin/sh")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

func (dn *DockerNode) GetAttr(name string) (string, error) {
	switch name {
	case "ifname":
		return dn.getInterfaceName()
	default:
		return dn.LocalNode.GetAttr(name)
	}
}

func (dn *DockerNode) SetAttr(name, val string) error {
	switch name {
	case "latency":
		return dn.setLatency(val)
	case "bandwidth":
		return dn.setBandwidth(val)
	case "jitter":
		return dn.setJitter(val)
	case "loss":
		return dn.setPacketLoss(val)
	default:
		return fmt.Errorf("no attribute named: %s", name)
	}
}

func (dn *DockerNode) setLatency(val string) error {
	dur, err := time.ParseDuration(val)
	if err != nil {
		return err
	}

	ifn, err := dn.getInterfaceName()
	if err != nil {
		return err
	}

	settings := &cnet.LinkSettings{
		Latency: uint(dur.Nanoseconds() / 1000000),
	}

	return cnet.SetLink(ifn, settings)
}

func (dn *DockerNode) setJitter(val string) error {
	dur, err := time.ParseDuration(val)
	if err != nil {
		return err
	}

	ifn, err := dn.getInterfaceName()
	if err != nil {
		return err
	}

	settings := &cnet.LinkSettings{
		Jitter: uint(dur.Nanoseconds() / 1000000),
	}

	return cnet.SetLink(ifn, settings)
}

// set bandwidth (expects Mbps)
func (dn *DockerNode) setBandwidth(val string) error {
	bw, err := strconv.ParseFloat(val, 32)
	if err != nil {
		return err
	}

	ifn, err := dn.getInterfaceName()
	if err != nil {
		return err
	}

	settings := &cnet.LinkSettings{
		Bandwidth: uint(bw * 1000000),
	}

	return cnet.SetLink(ifn, settings)
}

// set packet loss percentage (dropped / total)
func (dn *DockerNode) setPacketLoss(val string) error {
	ratio, err := strconv.ParseUint(val, 10, 8)
	if err != nil {
		return err
	}

	ifn, err := dn.getInterfaceName()
	if err != nil {
		return err
	}

	settings := &cnet.LinkSettings{
		PacketLoss: uint8(ratio),
	}

	return cnet.SetLink(ifn, settings)
}

func (dn *DockerNode) getInterfaceName() (string, error) {
	out, err := dn.RunCmd("ip", "link")
	if err != nil {
		return "", err
	}

	var cside string
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "@if") {
			ifnum := strings.Split(strings.Split(l, " ")[1], "@")[1]
			cside = ifnum[2 : len(ifnum)-1]
			break
		}
	}

	if cside == "" {
		return "", fmt.Errorf("container-side interface not found")
	}

	localout, err := exec.Command("ip", "link").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %s", err, localout)
	}

	for _, l := range strings.Split(string(localout), "\n") {
		if strings.HasPrefix(l, cside+": ") {
			return strings.Split(strings.Fields(l)[1], "@")[0], nil
		}
	}

	return "", fmt.Errorf("could not determine interface")
}
