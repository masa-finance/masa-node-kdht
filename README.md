# Masa Node kDHT

This is a simple node for libp2p using the kDHT.

## Installation

Before running the node, make sure you have Go installed on your machine. If you don't have Go installed, you can download it from the [official website](https://golang.org/dl/).

Once you have Go installed, you can download the necessary dependencies by running the command

```go get -d ./...```

After downloading the dependencies, you can build the project by running the command 

`go build`

## Running the Node

After downloading the dependencies, you can build the project by running:

To run the node, follow the steps below:

1. Run the boot node by executing the binary without any arguments:

`./masa-node-kdht`.

2. For the second, third, and subsequent nodes, you need to get the multiAddress from the log of the boot node and add it as an argument to the command. For example: 

`./masa-node-kdht /ip4/10.0.0.18/tcp/65189/p2p/16Uiu2HAm1WbqfBC645TY9W7X179vLecnKFqxY2pHrKGsAkuPXTEy`

