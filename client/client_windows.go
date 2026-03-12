//go:build windows

package main

const icmpProto string = "ip4:icmp"

const commonError string = "Make sure you are running with elevated " +
	"privileges and the target IP is reachable"
