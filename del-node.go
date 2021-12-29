package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/codegangsta/cli"
)

// del-node        host:port node_id
var delNodeCommand = cli.Command{
	Name:        "del-node",
	Aliases:     []string{"del"},
	Usage:       "del a redis node from existed cluster.",
	ArgsUsage:   `host:port node_id`,
	Description: `The del-node command delete a node from redis cluster.`,
	Action: func(context *cli.Context) error {
		if context.NArg() != 2 {
			fmt.Printf("Incorrect Usage.\n\n")
			cli.ShowCommandHelp(context, "del-node")
			logrus.Fatalf("Must provide \"host:port node_id\" for del-node command!")
		}

		rt := NewRedisTrib()
		if err := rt.DelNodeClusterCmd(context); err != nil {
			return err
		}
		return nil
	},
}

func (self *RedisTrib) DelNodeClusterCmd(context *cli.Context) error {
	var addr string
	var nodeid string

	if addr = context.Args().Get(0); addr == "" {
		return errors.New("please check host:port for del-node command")
	} else if nodeid = context.Args().Get(1); nodeid == "" {
		return errors.New("please check node_id for del-node command")
	}

	nodeid = strings.ToLower(nodeid)
	logrus.Printf(">>> Removing node %s from cluster %s", nodeid, addr)

	// Load cluster information
	if err := self.LoadClusterInfoFromNode(addr); err != nil {
		return err
	}

	// Check if the node exists and is not empty
	node := self.GetNodeByName(nodeid)
	if node == nil {
		logrus.Fatalf("No such node ID %s", nodeid)
	}

	if len(node.Slots()) > 0 {
		logrus.Fatalf("Node %s is not empty! Reshard data away and try again.", node.String())
	}
	// Send CLUSTER FORGET to all the nodes but the node to remove
	logrus.Printf(">>> Sending CLUSTER FORGET messages to the cluster...")
	for _, n := range self.Nodes() {
		if n == nil || n == node {
			continue
		}

		if n.Replicate() != "" && strings.ToLower(n.Replicate()) == nodeid {
			master := self.GetMasterWithLeastReplicas()
			if master != nil {
				logrus.Printf(">>> %s as replica of %s", n.String(), master.String())
				if _, err := n.ClusterReplicateWithNodeID(master.Name()); err != nil {
					logrus.Errorf("%s", err.Error())
				}
			}
		}

		if _, err := n.ClusterForgetNodeID(nodeid); err != nil {
			logrus.Errorf("%s", err.Error())
		}
	}
	// Finally shutdown the node
	logrus.Printf(">>> SHUTDOWN the node.")
	if err := node.ClusterNodeShutdown(); err != nil {
		return err
	}
	return nil
}
