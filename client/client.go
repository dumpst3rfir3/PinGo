package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/mjwhitta/cli"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

var flags struct {
	blockSize int
	dirPath   string
	filePath  string
	targetIP  string
}

func init() {
	cli.Flag(
		&flags.blockSize,
		"b",
		"block",
		1000,
		"Block size (in bytes) per ICMP packet",
	)
	cli.Flag(
		&flags.dirPath,
		"d",
		"directory",
		"",
		"Path to directory with files to send - NOTE: you cannot"+
			" pass both a directory AND a file",
	)
	cli.Flag(
		&flags.filePath,
		"f",
		"file",
		"",
		"Path to the file to send - NOTE: you cannot pass both a"+
			" directory AND a file",
	)
	cli.Flag(
		&flags.targetIP,
		"t",
		"target",
		"",
		"IP address of the target server where the data will be sent",
	)
	cli.Parse()
	if flags.targetIP == "" {
		cli.Usage(1)
	} else if flags.dirPath == "" && flags.filePath == "" {
		cli.Usage(1)
	} else if flags.dirPath != "" && flags.filePath != "" {
		cli.Usage(1)
	}
}

func main() {
	var err error

	if flags.dirPath != "" {
		err = sendDir(flags.dirPath)
	} else {
		err = sendFile(flags.filePath)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] Transfer failed: %v\n", err)
		fmt.Println(
			"[!] " + commonError,
		)
		os.Exit(1)
	}

	fmt.Println("[*] WOOOOOOO! Sending is complete. Have a nice day.")
}

func sendDir(dirPath string) error {
	return filepath.WalkDir(
		filepath.Clean(dirPath),
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			return sendFile(path)
		},
	)
}

func sendFile(filePath string) error {
	var (
		blockNum       int
		buf            []byte
		conn           *icmp.PacketConn
		dst            *net.UDPAddr
		err            error
		f              *os.File
		fileNameToSend string
		info           os.FileInfo
		msg            icmp.Message
		msgBytes       []byte
		n              int
		payload        []byte
		totalBlocks    int
	)

	info, err = os.Stat(filePath)
	if err != nil {
		fmt.Println("[!] Error with input file")
		return err
	}

	totalBlocks = int(info.Size()) / flags.blockSize
	if int(info.Size())%flags.blockSize != 0 {
		totalBlocks++
	}

	fmt.Printf(
		"[*] Sending %s (%d bytes) to %s\n",
		info.Name(), info.Size(), flags.targetIP,
	)
	fmt.Printf(
		"[*] Block size: %d bytes | Total blocks: %d\n",
		flags.blockSize, totalBlocks,
	)

	f, err = os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	conn, err = icmp.ListenPacket(icmpProto, "0.0.0.0")
	if err != nil {
		return err
	}
	defer conn.Close()

	dst, err = net.ResolveUDPAddr(icmpProto, flags.targetIP+":0")
	if err != nil {
		return err
	}

	// Packet format: [blockNum uint32][totalBlocks uint32][data...]
	// Block 0 is the filename packet; blocks 1..N are file data.

	// Send block 0: filename packet
	if flags.filePath != "" {
		fileNameToSend = filepath.ToSlash(filepath.Base(filePath))
	} else {
		fileNameToSend, err = filepath.Rel(flags.dirPath, filePath)
		fileNameToSend = filepath.ToSlash(fileNameToSend)
		if err != nil {
			return err
		}
	}

	payload = make([]byte, 8+len(fileNameToSend))
	binary.BigEndian.PutUint32(payload[0:4], 0)
	binary.BigEndian.PutUint32(payload[4:8], uint32(totalBlocks))
	copy(payload[8:], []byte(fileNameToSend))

	msg = icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   os.Getpid() & 0xffff,
			Seq:  0,
			Data: payload,
		},
	}

	msgBytes, err = msg.Marshal(nil)
	if err != nil {
		return err
	}

	_, err = conn.WriteTo(msgBytes, dst)
	if err != nil {
		return err
	}

	fmt.Printf("[*] Sent filename: %s\n", fileNameToSend)

	buf = make([]byte, flags.blockSize)
	blockNum = 1

	for {
		n, err = f.Read(buf)
		if n == 0 {
			break
		}
		if err != nil && err != io.EOF {
			return err
		}

		payload = make([]byte, 8+n)
		binary.BigEndian.PutUint32(
			payload[0:4], uint32(blockNum),
		)
		binary.BigEndian.PutUint32(
			payload[4:8], uint32(totalBlocks),
		)
		copy(payload[8:], buf[:n])

		msg = icmp.Message{
			Type: ipv4.ICMPTypeEcho,
			Code: 0,
			Body: &icmp.Echo{
				ID:   os.Getpid() & 0xffff,
				Seq:  blockNum & 0xffff,
				Data: payload,
			},
		}

		msgBytes, err = msg.Marshal(nil)
		if err != nil {
			return err
		}

		_, err = conn.WriteTo(msgBytes, dst)
		if err != nil {
			return err
		}

		fmt.Printf(
			"\r[*] Sent block %d of %d",
			blockNum, totalBlocks,
		)
		blockNum++
		time.Sleep(time.Millisecond)
	}

	fmt.Println()
	return nil
}
