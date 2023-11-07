package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/joho/godotenv"
)

type Block struct {
	Index     int
	Timestamp string
	BPM       int
	Hash      string
	PrevHash  string
	Validator string
}

var (
	Blockchain      []Block
	tempBlocks      []Block
	candidateBlocks = make(chan Block)
	announcements   = make(chan string)
	mutex           = &sync.Mutex{}
	validators = make(map[string]int)
)

func main() {
	err := godotenv.Load()
	if err != nil{
		log.Fatal(err)
	}

	t := time.Now()
	genesisBlock := Block{}
	genesisBlock = Block{Timestamp: t.String(), Hash: calcuateBlockHash(genesisBlock)}
	spew.Dump(genesisBlock)
	Blockchain = append(Blockchain, genesisBlock)

	tcpPort := os.Getenv("PORT")

	server, err := net.Listen("tcp", ":" + tcpPort)
	if err != nil{
		log.Fatal(err)
	}
	log.Println("TCP Server Listening on port :", tcpPort)
	defer server.Close()

	go func(){
		for candidate := range candidateBlocks{
			mutex.Lock()
			tempBlocks = append(tempBlocks, candidate)
			mutex.Unlock()
		}
	}()

	go func ()  {
		for {
			pickWinner()
		}
	}()

	for {
		conn, err := server.Accept()
		if err != nil{
			log.Fatal(err)
		}
		go handleConn(conn)
	}
}

func pickWinner(){
	time.Sleep(30 * time.Second)
	mutex.Lock()
	temp := tempBlocks
	mutex.Unlock()

	lotteryPool := []string{}
	if len(temp) > 0{
		OUTER:
			for _, block := range temp{
				for _, node := range lotteryPool {
					if block.Validator == node{
						continue OUTER
					}
				}
				mutex.Lock()
				setValidators := validators
				mutex.Unlock()

				k, ok := setValidators[block.Validator]
				if ok {
					for i := 0; i < k; i++ {
						lotteryPool = append(lotteryPool, block.Validator)
				}
			}
		}
		s := rand.NewSource(time.Now().Unix())
		r := rand.New(s)
		lotteryWinner := lotteryPool[r.Intn(len(lotteryPool))]

		for _, block := range temp {
			if block.Validator == lotteryWinner {
				mutex.Lock()
				Blockchain = append(Blockchain, block)
				mutex.Unlock()
				for _ = range validators {
					announcements <- "\nwinning validator: " + lotteryWinner + "\n"
				}
				break
			}
		}	
	}
	mutex.Lock()
	tempBlocks = []Block{}
	mutex.Unlock()
}

func handleConn(conn net.Conn) {
	defer conn.Close()

	go func() {
		for {
			msg := <- announcements
			io.WriteString(conn, msg)
		}
	}()
	var address string

	io.WriteString(conn, "Enter token balance:")
	scanBalance := bufio.NewScanner(conn)
	for scanBalance.Scan() {
		balance, err := strconv.Atoi(scanBalance.Text())
		if err != nil{
			log.Printf("%v not a number: %v", scanBalance.Text(), err)
			return
		}
		t := time.Now()
		address = calcuateHash(t.String())
		validators[address] = balance
		fmt.Println(validators)
		break
	}

	io.WriteString(conn, "\nEnter a new BPM:")

	scanBPM := bufio.NewScanner(conn)

	go func() {
		for {
			for scanBPM.Scan() {
				bpm, err := strconv.Atoi(scanBPM.Text())
				if err != nil{
					log.Printf("%v not a number: %v", scanBPM.Text(), err)
					delete(validators, address)
					conn.Close()
				}
				mutex.Lock()
				oldLastIndex := Blockchain[len(Blockchain)-1]
				mutex.Unlock()

				newBlock, err := generateBlock(oldLastIndex, bpm, address)
				if err != nil{
					log.Println(err)
					continue
				}
				if isBlockValid(newBlock, oldLastIndex){
					candidateBlocks <-newBlock
				}
				io.WriteString(conn, "\nEnter a new BPM:")
			}	
		}
	}()

	for {
		time.Sleep(time.Minute)
		mutex.Lock()
		output, err := json.Marshal(Blockchain)
		mutex.Unlock()
		if err != nil{
			log.Fatal(err)
		}
		io.WriteString(conn, string(output) + "\n")
	}
}

func isBlockValid(newBlock, oldBlock Block) bool{
	if oldBlock.Index+1 != newBlock.Index{
		return false
	}
	if oldBlock.Hash != newBlock.PrevHash{
		return false
	}
	if calcuateBlockHash(newBlock) != newBlock.Hash{
		return false
	}
	return true
}

func calcuateHash(s string) string{
	h := sha256.New()
	h.Write([]byte(s))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

func calcuateBlockHash(block Block) string{
	record := string(block.Index) + block.Timestamp + string(block.BPM) + block.PrevHash
	return calcuateHash(record)
}

func generateBlock(oldBlock Block, BPM int, address string) (Block, error) {
	var newBlock Block
	t := time.Now()
	newBlock.Index = oldBlock.Index + 1
	newBlock.Timestamp = t.String()
	newBlock.BPM = BPM
	newBlock.PrevHash = oldBlock.Hash
	newBlock.Hash = calcuateBlockHash(newBlock)
	newBlock.Validator = address

	return newBlock, nil
}