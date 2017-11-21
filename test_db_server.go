package main

import (
	"flag"
	"fmt"
	"github.com/tarantool/go-tarantool"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

const (
	LOAD_COUNT = 100000
)

/* ----- */

type Flags struct {
	conc   int
	total  int
	keylen int
}

func usage() {
	fmt.Printf("Usage %s subs ping change\n", filepath.Base(os.Args[0]))
	os.Exit(1)
}

type WorkerFunc func(*tarantool.Connection, int, int, *Progress)

type Progress struct {
	total int32
	count int32

	last_since time.Time
	last_count int32
}

func NewProgress(total int) *Progress {
	return &Progress{total: int32(total)}
}

func (p *Progress) increment() {
	new_count := atomic.AddInt32(&p.count, 1)

	if new_count == 1 {
		p.last_since = time.Now()
		p.last_count = 0
	} else if p.total > 100 && (new_count%(p.total/10)) == 0 {
		now := time.Now()
		elapsed := now.Sub(p.last_since).Seconds()
		rps := float64(new_count-p.last_count) / elapsed
		p.last_since = now
		p.last_count = new_count
		log.Printf("Completed %6d requests, %9.2f rps\n", new_count, rps)
	}
}

/* ----- */

type DevFolderSub struct {
	dev_id    string
	folder_id string
	sub_id    string
}

func NewDevFolderSub_DbEnt(ent SubEnt) DevFolderSub {
	return DevFolderSub{dev_id: ent.dev_id, folder_id: ent.folder_id, sub_id: ent.sub_id}
}

func NewDevFolderSub_Vars(dev_id string, folder_id string, sub_id string) DevFolderSub {
	return DevFolderSub{dev_id: dev_id, folder_id: folder_id, sub_id: sub_id}
}

var LIST_ENTS []DevFolderSub = nil
var LIST_MUTEX sync.Mutex

func loadDevicesAndSubs(client *tarantool.Connection, numreq int) ([]DevFolderSub, int) {

	schema := client.Schema

	space_subs := schema.Spaces["subs"]
	index_subs_primary := space_subs.Indexes["primary"]

	LIST_MUTEX.Lock()
	defer LIST_MUTEX.Unlock()

	needLoad := LIST_ENTS == nil
	if needLoad {
		log.Println("Loading sub and device ids")

		LIST_ENTS = make([]DevFolderSub, 0, LOAD_COUNT)

		var subs []SubEnt
		err := client.SelectTyped(space_subs, index_subs_primary, 0, LOAD_COUNT, tarantool.IterAll, []interface{}{}, &subs)
		if err != nil {
			log.Fatalf("Error calling select: %s", err)
		}

		for _, sub := range subs {
			LIST_ENTS = append(LIST_ENTS, NewDevFolderSub_DbEnt(sub))
		}
	}

	ent_count := len(LIST_ENTS)

	if needLoad {
		log.Printf("Loaded %d ents", ent_count)
	}

	if numreq > ent_count {
		if needLoad {
			log.Printf("Limiting %d iterations to %d existing ents", numreq, ent_count)
		}
		numreq = ent_count
	}

	return LIST_ENTS, numreq
}

/* ----- */

func runFuncSubs(client *tarantool.Connection, keylen int, numreq int, p *Progress) {

	// Create and save devices and subscriptions
	list_ents := make([]DevFolderSub, 0, numreq)

	model := NewPushDbModel(client)

	for i := 0; i < numreq; i++ {
		dev_id := genRandomString(keylen)
		auth := genRandomString(AUTH_STRING_LEN)
		push_token := genPushToken()
		now := milliTime()

		// Model
		t_dev, code, err := model.doCreateDev(dev_id, auth, push_token, PUSH_TECH_GCM_DEBUG, now)
		if t_dev == nil || code != RES_OK || err != nil {
			log.Fatalf("Error calling function: %s", err)
		}

		// Model
		folder_id := fmt.Sprintf("%08d", i)
		sub_id := genRandomString(keylen)
		code, err = model.doCreateSub(dev_id, folder_id, sub_id, now)
		if code != RES_OK || err != nil {
			log.Fatalf("Error calling function: %s", err)
		}

		p.increment()

		list_ents = append(list_ents, NewDevFolderSub_Vars(dev_id, folder_id, sub_id))
	}

	// Add more subscriptions to the devices
	for i := 0; i < numreq; i++ {
		dev_id := list_ents[i].dev_id

		// Model
		folder_id := fmt.Sprintf("%08d", numreq+i)
		sub_id := genRandomString(keylen)
		now := milliTime()
		code, err := model.doCreateSub(dev_id, folder_id, sub_id, now)
		if code != RES_OK || err != nil {
			log.Fatalf("Error calling function: %d, %s", code, err)
		}
		p.increment()

		// Model
		folder_id = fmt.Sprintf("%08d", numreq+numreq+i)
		sub_id = genRandomString(keylen)
		now = milliTime()
		code, err = model.doCreateSub(dev_id, folder_id, sub_id, now)
		if code != RES_OK || err != nil {
			log.Fatalf("Error calling function: %d, %s", code, err)
		}
		p.increment()
	}

	LIST_MUTEX.Lock()
	defer LIST_MUTEX.Unlock()

	if LIST_ENTS == nil {
		LIST_ENTS = make([]DevFolderSub, 0, numreq)
	}
	for i, ent := range list_ents {
		LIST_ENTS = append(LIST_ENTS, ent)
		if i == numreq {
			break
		}
	}
}

func runFuncPing(client *tarantool.Connection, keylen int, numreq int, p *Progress) {
	list_ents, numreq := loadDevicesAndSubs(client, numreq)
	size_ents := len(list_ents)

	model := NewPushDbModel(client)

	for i := 0; i < numreq; i++ {
		index := rand.Intn(size_ents)
		ent := list_ents[index]
		dev_id := ent.dev_id
		folder_id := ent.folder_id
		sub_id := ent.sub_id

		ping_ts := milliTime()

		// Model
		code, err := model.doPingSub(dev_id, folder_id, sub_id, ping_ts)
		if code != RES_OK || err != nil {
			log.Fatalf("Error calling function: %d, %s", code, err)
		}

		p.increment()
	}
}

func runFuncChange(client *tarantool.Connection, keylen int, numreq int, p *Progress) {
	priority := false

	list_ents, numreq := loadDevicesAndSubs(client, numreq)
	size_ents := len(list_ents)

	model := NewPushDbModel(client)

	for i := 0; i < numreq; i++ {
		index := rand.Intn(size_ents)
		ent := list_ents[index]
		dev_id := ent.dev_id
		folder_id := ent.folder_id
		sub_id := ent.sub_id

		change_ts := milliTime()
		delta := TIME_MS_500_MILLIS

		// Model
		code, err := model.doChangeSub(dev_id, folder_id, sub_id, change_ts, delta, priority)
		if code != RES_OK || err != nil {
			log.Fatalf("Error calling function: %d, %s", code, err)
		}

		p.increment()
	}
}

func runHarness(flags Flags, client *tarantool.Connection, worker WorkerFunc) {
	rand.Seed(time.Now().UTC().UnixNano())

	log.Printf("Key length: %d\n", flags.keylen)

	var wg sync.WaitGroup
	wg.Add(flags.conc)

	now := time.Now()
	progress := NewProgress(flags.total)

	for i := 0; i < flags.conc; i++ {
		numreq := flags.total / flags.conc
		keylen := flags.keylen
		go func(keylen, numreq int) {
			defer wg.Done()
			worker(client, keylen, numreq, progress)
		}(keylen, numreq)
	}

	wg.Wait()

	since := time.Since(now)

	log.Printf("Elapsed time: %s\n", since)
	log.Printf("Ops per second: %.2f\n", float64(progress.count)/since.Seconds())
}

/* ----- */

func main() {
	// Flags
	var flags Flags
	flag.IntVar(&flags.conc, "c", 20, "Concurrency")
	flag.IntVar(&flags.total, "n", 100000, "Total count")
	flag.IntVar(&flags.keylen, "l", 40, "Key length")
	flag.Parse()

	nargs := flag.NArg()
	args := flag.Args()

	if nargs < 1 {
		usage()
	}
	if flags.conc < 1 || flags.total < 1 || flags.keylen < 10 {
		usage()
	}

	// Config
	config, err := NewDbConfig()
	if err != nil {
		fmt.Printf("Fatal config error: %s\n", err)
		os.Exit(1)
	}

	// Database connection, need only one
	var client *tarantool.Connection

	// Run commands
	for _, command := range args {
		// Connect if needed
		if client == nil {
			client_init, err := config.Connect(config.Bind)
			if err != nil {
				log.Fatalf("Failed to connect: %s", err)
			}

			client = client_init
		}

		// Adjust concurrency
		if flags.total <= 100 && flags.conc > 1 {
			fmt.Printf("Total is small, using one thread\n")
			flags.conc = 1
		}

		if command == "subs" {
			log.Printf("Subs test, c = %d, n = %d\n", flags.conc, flags.total)
			runHarness(flags, client, runFuncSubs)
		} else if command == "ping" {
			log.Printf("Ping test, c = %d, n = %d\n", flags.conc, flags.total)
			runHarness(flags, client, runFuncPing)
		} else if command == "change" {
			log.Printf("Change test, c = %d, n = %d\n", flags.conc, flags.total)
			runHarness(flags, client, runFuncChange)
		} else {
			usage()
		}
	}
}
