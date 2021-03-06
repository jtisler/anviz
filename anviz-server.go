package main

import (
	"fmt"
	"net"
	"log"
	"encoding/json"
	"encoding/hex"
	"bytes"
	"time"
	"os"
	"github.com/mikespook/gearman-go/worker"
	"strconv"
)

type jsonReq struct {
	Id int
	Port string
	Command []string
	Type int
}

func handleConnection(port string) (net.Conn, error) {

	portFixed, _ := strconv.Atoi(port[1:])

	ln, err := net.ListenTCP("tcp", &net.TCPAddr{
		//IP: net.IPv4(0,0,0,0),
		Port: portFixed,
	}) //start listening od desired port

	if err != nil { //check errors
		log.Fatal(err.Error())
		return nil, err
	}

	ln.SetDeadline(time.Now().Add( 5 * time.Second ))

	conn, connErr := ln.Accept() //accept first (and only) device that connects

	if connErr, ok := connErr.(net.Error); ok && connErr.Timeout() {
  		return nil, connErr
	}

	log.Print("New connection from " + conn.RemoteAddr().String())

	if err := ln.Close(); err != nil { //check errors
		log.Fatal(err.Error())
		return nil, err
	}


	return conn, nil
}

/*

	create request formatted as:

	STX	CH(device code)		CMD(command)	LEN(data length)	DATA		CRC16
	0xA5	4 bytes			1Byte		2 Bytes			0-400 Bytes	2 Bytes

*/


func parseResponse(response []byte) string{
	sResponse := fmt.Sprintf("%X", response) //convert []byte to hex string

	return sResponse
}

func executeSingle(conn net.Conn, command []string) ([]byte, error) {
	request, _ := hex.DecodeString(command[0])

	conn.Write(request) //send request to device

	response := make([]byte, 4096) //reserve 4096 byte buffer for read

	_, err := conn.Read(response) //get response from device

	if err != nil { //check errors
		log.Fatal(err.Error())
	 	return nil, err
	}

	response = bytes.Trim(response, "\x00") //trim trailing zeroes

	pResponse := parseResponse(response)

	return []byte(pResponse), nil
}

func executeMulti(conn net.Conn, command []string) ([]byte, error) {

	var sResponse = make([]string, len(command))

	for v := range command {
		request, _ := hex.DecodeString(command[v])

		conn.Write(request) //send request to device

		response := make([]byte, 4096) //reserve 4096 byte buffer for read

		_, err := conn.Read(response) //get response from device

		if err != nil { //check errors
			log.Fatal(err.Error())
			return nil, err
		}

		response = bytes.Trim(response, "\x00") //trim trailing zeroes

		pResponse := parseResponse(response)

		response = make([]byte, 4096)

		sResponse[v] = pResponse //parse response
	}

	jResponse, _ := json.Marshal(sResponse)

	return jResponse, nil
}

func Anviz(job worker.Job) ([]byte, error){
	var req jsonReq //new json request

	json.Unmarshal(job.Data(), &req) //unmarshal data and map it to request variable

	conn, err := handleConnection(req.Port) //create connection

	if err != nil { //if error while establishing connection get stop job
		//log.Fatal(err.Error())
		return []byte(err.Error()), nil
	}

	defer conn.Close();

	fmt.Println("connection established")


	var response []byte //initialize response variable

	if req.Type == 1 {
		response, _ = executeMulti(conn, req.Command) //call execute command
	} else {
		response, _ = executeSingle(conn, req.Command)
	}

	return response, nil //and return it's response
}


func main(){
	f, _ := os.OpenFile("/var/log/directa.log", os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)

	defer f.Close()

	log.SetOutput(f)

	w := worker.New(worker.Unlimited) //initialize new worker
	defer w.Close() //defer worker close

	w.AddServer("tcp", "0.0.0.0:4730") //starts listening on port 4730
	w.AddFunc("Anviz", Anviz, worker.Unlimited) //add function anviz

	if err := w.Ready(); err != nil { //check if error while creating worker stop
		log.Fatal(err)
		return
	}

	w.Work() //start worker
}
