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
	"github.com/jtisler/directa/crc16"
	"github.com/mikespook/gearman-go/worker"
)

type jsonReq struct {
	Id int
	Port string
	Command string
	Data string
	Length int
}

func handleConnection(port string) (net.Conn, error) {
	ln, err := net.Listen("tcp", port) //start listening od desired port

	if err != nil { //check errors
		log.Fatal(err.Error())
		return nil, err
	}

	conn, _ := ln.Accept() //accept first (and only) device that connects

	log.Print("New connection from " + conn.RemoteAddr().String())

	if err := ln.Close(); err != nil { //check errors
		log.Fatal(err.Error())
		return nil, err
	}

	conn.SetDeadline(time.Now().Add(15 * time.Second)) //set connection read-write timeout

	return conn, nil
}

/*

	create request formatted as:

	STX	CH(device code)		CMD(command)	LEN(data length)	DATA		CRC16
	0xA5	4 bytes			1Byte		2 Bytes			0-400 Bytes	2 Bytes

*/


func buildRequest(id int, command string, data string, length int) []byte {
	sReq := "a5" //always start with 15
	sReq = fmt.Sprintf("%s%08d%02s%04X%s", sReq, id, command, length, data) //concat params

	bReq, _ := hex.DecodeString(sReq) //convert hex string to []byte
	sCrc_16 := crc16.Crc16(bReq) //calculate crc_16

	sReq = fmt.Sprintf("%s%s", sReq, sCrc_16) //concat request and crc_16

	bReq, _ = hex.DecodeString(sReq) //convert hex string to []byte

	return bReq
}

func parseResponse(response []byte) string{
	sResponse := fmt.Sprintf("%X", response) //convert []byte to hex string

	return sResponse
}

func executeCommand(conn net.Conn, id int, command string, data string, length int) string {

	request := buildRequest(id, command, data, length) //build request
	conn.Write(request) //send request to device

	response := make([]byte, 2048) //reserve 2048 byte buffer for read
	_, err := conn.Read(response) //get response from device

	if err != nil { //check errors
		log.Fatal(err.Error())
		return err.Error()
	}

	response = bytes.Trim(response, "\x00") //trim trailing zeroes
	sResponse := parseResponse(response) //parse response

	return sResponse
}

func Anviz(job worker.Job) ([]byte, error){
	var req jsonReq //new json request

	json.Unmarshal(job.Data(), &req) //unmarshal data and map it to request variable

	conn, err := handleConnection(req.Port) //create connection

	defer conn.Close() //defer connection close

	if err != nil { //if error while establishing connection get stop job
		log.Fatal(err.Error())
		return nil, err
	}

	var response string //initialize response variable

	response = executeCommand(conn, req.Id, req.Command, req.Data, req.Length) //call execute command

	return []byte(response), nil //and return it's response
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
