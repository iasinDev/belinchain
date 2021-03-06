package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"
    "fmt"
    "sync"
    "strings"
	"github.com/davecgh/go-spew/spew"
    "github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

const difficulty = 1

// Block represents each 'item' in the blockchain
type Block struct {
	Index       int
	Timestamp   string
	Data        string
	Hash        string
	PrevHash    string
    Difficulty  int
    Nonce       string
}

// Blockchain is a series of validated Blocks
var Blockchain []Block

type Message struct {
	Data int
}

var mutex = &sync.Mutex{}

// SHA256 hashing
func calculateHash(block Block) string {
	record := strconv.Itoa(block.Index) + block.Timestamp + block.Data + block.PrevHash + block.Nonce
	h := sha256.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

// create a new block using previous block's hash
func generateBlock(oldBlock Block, Data string) (Block, error) {
        var newBlock Block

        t := time.Now()

        newBlock.Index = oldBlock.Index + 1
        newBlock.Timestamp = t.String()
        newBlock.Data = Data
        newBlock.PrevHash = oldBlock.Hash
        newBlock.Difficulty = difficulty

        for i := 0; ; i++ {
                hex := fmt.Sprintf("%x", i)
                newBlock.Nonce = hex
                if !isHashValid(calculateHash(newBlock), newBlock.Difficulty) {
                        fmt.Println(calculateHash(newBlock), " do more work!")
                        time.Sleep(time.Second)
                        continue
                } else {
                        fmt.Println(calculateHash(newBlock), " work done!")
                        newBlock.Hash = calculateHash(newBlock)
                        break
                }

        }
        return newBlock, nil
}

// make sure block is valid by checking index, and comparing the hash of the previous block
func isBlockValid(newBlock, oldBlock Block) bool {
	if oldBlock.Index+1 != newBlock.Index {
		return false
	}

	if oldBlock.Hash != newBlock.PrevHash {
		return false
	}

	if calculateHash(newBlock) != newBlock.Hash {
		return false
	}

	return true
}


// make sure the chain we're checking is longer than the current blockchain
func replaceChain(newBlocks []Block) {
	if len(newBlocks) > len(Blockchain) {
		Blockchain = newBlocks
	}
}

func isHashValid(hash string, difficulty int) bool {
        prefix := strings.Repeat("0", difficulty)
        return strings.HasPrefix(hash, prefix)
}

// bcServer handles incoming concurrent Blocks
var bcServer chan []Block


func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}

	bcServer = make(chan []Block)

	// create genesis block
	t := time.Now()
    genesisBlock := Block{}
	genesisBlock = Block{0, t.String(), "0", calculateHash(genesisBlock), "", difficulty, ""} 
	spew.Dump(genesisBlock)
	Blockchain = append(Blockchain, genesisBlock)
    
    // start TCP and serve TCP server
	server, err := net.Listen("tcp", ":"+"9000")
	if err != nil {
		log.Fatal(err)
	}
	defer server.Close()
    
    // start HTTP server
    go run()
    
	for {
		conn, err := server.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handleConn(conn)
	}
    
}


///////////////// TCP ////////////////////////////

func handleConn(conn net.Conn) {
	defer conn.Close()
    
    io.WriteString(conn, "Enter Data:")

	scanner := bufio.NewScanner(conn)

	// take in BPM from stdin and add it to blockchain after conducting necessary validation
	go func() {
		for scanner.Scan() {
			data := scanner.Text()

            mutex.Lock()
			newBlock, err := generateBlock(Blockchain[len(Blockchain)-1], data)
            mutex.Unlock()
            
			if err != nil {
				log.Println(err)
				continue
			}
			if isBlockValid(newBlock, Blockchain[len(Blockchain)-1]) {
				newBlockchain := append(Blockchain, newBlock)
				replaceChain(newBlockchain)
			}

			bcServer <- Blockchain
			io.WriteString(conn, "\nEnter Data:")
		}
	}()

	for _ = range bcServer {
		spew.Dump(Blockchain)
	}

}

///////////////// http ////////////////////////////

func run() error {
	mux := makeMuxRouter()
	httpAddr := "8080" //os.Getenv("ADDR")
	log.Println("Listening on ", "8080")
	s := &http.Server{
		Addr:           ":" + httpAddr,
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	if err := s.ListenAndServe(); err != nil {
		return err
	}

	return nil
}

func makeMuxRouter() http.Handler {
	muxRouter := mux.NewRouter()
	muxRouter.HandleFunc("/", handleGetBlockchain).Methods("GET")
	return muxRouter
}

func handleGetBlockchain(w http.ResponseWriter, r *http.Request) {
	bytes, err := json.MarshalIndent(Blockchain, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	io.WriteString(w, string(bytes))
}



