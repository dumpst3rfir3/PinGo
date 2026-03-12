//go:build !windows

package main

const icmpProto string = "udp4"

const commonError string = "Make sure the target IP is reachable"
