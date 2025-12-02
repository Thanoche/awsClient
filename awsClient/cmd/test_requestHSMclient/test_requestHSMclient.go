package main

import (
	"flag"
	"fmt"
	"strconv"

	hsmClient "awsClient/pkg/requestHSMclient"
)

// address of the HSM client
const HSM_CLIENT_DEFAULT_PORT = 6123

func main() {
	// command-line arguments
	hsm_client_port_flag := flag.Int("HSMclient", HSM_CLIENT_DEFAULT_PORT, "HSM client port")
	flag.Parse()
	HSM_CLIENT_ADDRESS := "localhost:" + strconv.Itoa(*hsm_client_port_flag)

	// keys we want to retrieve on 2 different HSM
	keyHSM_1 := hsmClient.KeyHSM{
		Hsm_number: 40, // keystore key17
		Key_index:  1,  // key at index 1
	}
	keyHSM_2 := hsmClient.KeyHSM{
		Hsm_number: 41, // keystore key22
		Key_index:  1,  // key at index 1
	}

	// make parallel key requests
	key := hsmClient.GetKey(HSM_CLIENT_ADDRESS, keyHSM_1, keyHSM_2)

	// print result
	if len(key) == 0 {
		fmt.Println("Get key request failed.")
	} else {
		fmt.Println("key :")
		fmt.Println(key)
	}
}
