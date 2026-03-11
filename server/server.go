package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/mjwhitta/cli"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

var flags struct {
	outDir  string
	timeout int
}

// Error to handle the user entering CTRL+c
var errStop error = errors.New("stop")

func init() {
	cli.Flag(
		&flags.outDir,
		"o",
		"outDir",
		".",
		"Directory to write received files",
	)
	cli.Flag(
		&flags.timeout,
		"t",
		"timeout",
		30,
		"Seconds to wait after first block before timing out",
	)
	cli.Parse()
}

func main() {
	fmt.Printf(
		"[*] Listening for files (saving to %s)...\n",
		flags.outDir,
	)

	startReceiving()

	fmt.Println("[*] WOOOOOOOO! Have a nice day")
}

func recvFile(conn *icmp.PacketConn, stop *bool) error {
	var (
		blockNum    uint32
		blocks      map[uint32][]byte
		buf         []byte
		echo        *icmp.Echo
		err         error
		f           *os.File
		filename    string
		i           uint32
		missing     []uint32
		msg         *icmp.Message
		n           int
		ok          bool
		outPath     string
		received    int
		seen        map[uint32]bool
		totalBlocks uint32
	)

	// Clear any deadline left over from the previous file so we
	// wait indefinitely for the next file's block 0
	err = conn.SetReadDeadline(time.Time{})
	if err != nil {
		return err
	}

	blocks = make(map[uint32][]byte)
	buf = make([]byte, 65536)
	seen = make(map[uint32]bool)

	for {
		n, _, err = conn.ReadFrom(buf)
		if err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) {
				fmt.Println()
				// First check if "timeout" is due to CTRL+c (see the
				// check for CTRL+c above)
				if *stop {
					return errStop
				}
				// Otherwise, this is a true timeout
				fmt.Printf(
					"[!] Timeout after %d seconds\n",
					flags.timeout,
				)
				break
			}
			return err
		}

		msg, err = icmp.ParseMessage(1, buf[:n])
		if err != nil {
			continue
		}

		if msg.Type != ipv4.ICMPTypeEcho {
			continue
		}

		echo, ok = msg.Body.(*icmp.Echo)
		if !ok || len(echo.Data) < 8 {
			continue
		}

		// Packet format: [blockNum uint32][totalBlocks uint32][data...]
		// Block 0 is the filename packet; blocks 1..N are file data.
		blockNum = binary.BigEndian.Uint32(echo.Data[0:4])
		totalBlocks = binary.BigEndian.Uint32(echo.Data[4:8])

		// Check for duplicate packets
		if seen[blockNum] {
			continue
		}
		seen[blockNum] = true

		// Block 0 is just the file name
		if blockNum == 0 {
			filename = filepath.ToSlash(string(echo.Data[8:]))
			fmt.Printf("[*] Incoming file: %s\n", filename)

			// The timeout timer is not started until the first packet
			// of a file is received
			err = conn.SetReadDeadline(
				time.Now().Add(
					time.Duration(flags.timeout) * time.Second,
				),
			)
			if err != nil {
				return err
			}
			continue
		}

		data := make([]byte, len(echo.Data)-8)
		copy(data, echo.Data[8:])
		blocks[blockNum] = data
		received++

		fmt.Printf(
			"\r[*] Received block %d of %d",
			received, totalBlocks,
		)

		if uint32(received) == totalBlocks {
			fmt.Println()
			break
		}
	}

	if filename == "" {
		return fmt.Errorf("no filename received")
	}

	// Get rid of any `..`s in the path (e.g., to prevent dir traversal)
	for strings.HasPrefix(filename, "../") {
		filename = strings.TrimPrefix(filename, "../")
	}

	outPath = filepath.Join(flags.outDir, filename)

	// Create the dir(s), if necessary
	err = os.MkdirAll(filepath.Dir(outPath), 0o755)
	if err != nil {
		return err
	}

	f, err = os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// Check for missing blocks (e.g., due to a timeout)
	// If the current block is not missing, write it to file
	for i = 1; i <= totalBlocks; i++ {
		_, ok = blocks[i]
		if !ok {
			missing = append(missing, i)
			continue
		}

		_, err = f.Write(blocks[i])
		if err != nil {
			return err
		}
	}

	if len(missing) > 0 {
		fmt.Printf("[!] Missing blocks: %v\n", missing)
		return fmt.Errorf(
			"transfer incomplete: %d of %d blocks missing",
			len(missing), totalBlocks,
		)
	}

	fmt.Printf("[*] Saved to: %s\n", outPath)
	return nil
}

func startReceiving() {
	var (
		conn  *icmp.PacketConn
		err   error
		sigCh chan os.Signal
		stop  bool
	)

	conn, err = icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] Error: %v\n", err)
		return
	}
	defer conn.Close()

	// Create a channel to handle the user entering CTRL+c (os.Interrupt)
	sigCh = make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	// Check for CTRL+c
	go func() {
		<-sigCh
		stop = true
		// If CTRL+c is entered, set the timeout to now
		conn.SetReadDeadline(time.Now())
	}()

	for {
		err = recvFile(conn, &stop)
		if err != nil {
			// recvFile returns errStop when the user enters CTRL+c
			if errors.Is(err, errStop) {
				return
			}
			fmt.Fprintf(os.Stderr, "[!] Error: %v\n", err)
		}
	}
}
