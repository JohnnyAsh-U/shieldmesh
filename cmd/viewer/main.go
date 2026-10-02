package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/nats-io/nats.go"
)

func main() {
	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		log.Fatalf("Failed to get JetStream context: %v", err)
	}

	http.HandleFunc("/requests", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		info, err := js.StreamInfo("REQUEST_STREAM")
		if err != nil {
			http.Error(w, fmt.Sprintf("Error getting stream info: %v", err), http.StatusInternalServerError)
			return
		}

		var messages = make([]json.RawMessage, 0, 50)

		// Fetch up to the last 50 messages
		lastSeq := info.State.LastSeq
		firstSeq := info.State.FirstSeq

		if lastSeq-firstSeq > 49 {
			firstSeq = lastSeq - 49
		}

		for i := firstSeq; i <= info.State.LastSeq; i++ {
			msg, err := js.GetMsg("REQUEST_STREAM", i)
			if err == nil {
				messages = append(messages, msg.Data)
			}
		}

		json.NewEncoder(w).Encode(messages)
	})

	http.HandleFunc("/decisions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		info, err := js.StreamInfo("DECISION_STREAM")
		if err != nil {
			http.Error(w, fmt.Sprintf("Error getting stream info: %v", err), http.StatusInternalServerError)
			return
		}

		var messages = make([]json.RawMessage, 0, 50)

		// Fetch up to the last 50 messages
		lastSeq := info.State.LastSeq
		firstSeq := info.State.FirstSeq

		// fmt.Println(startSeq)
		if lastSeq-firstSeq > 49 {
			firstSeq = lastSeq - 49
		}

		for i := firstSeq; i <= info.State.LastSeq; i++ {
			msg, err := js.GetMsg("DECISION_STREAM", i)
			if err == nil {
				messages = append(messages, msg.Data)
			}
		}

		json.NewEncoder(w).Encode(messages)
	})

	http.HandleFunc("/engines", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		kv, err := js.KeyValue("SHIELDMESH_ENGINES")
		if err != nil {
			http.Error(w, fmt.Sprintf("Error accessing KV: %v", err), http.StatusInternalServerError)
			return
		}

		keys, err := kv.Keys()
		if err != nil && err != nats.ErrNoKeysFound {
			http.Error(w, fmt.Sprintf("Error fetching keys: %v", err), http.StatusInternalServerError)
			return
		}

		engines := make(map[string]json.RawMessage)
		for _, key := range keys {
			entry, err := kv.Get(key)
			if err == nil {
				engines[key] = entry.Value()
			}
		}

		json.NewEncoder(w).Encode(engines)
	})

	http.HandleFunc("/clear", func(w http.ResponseWriter, r *http.Request) {
		// Purge streams
		js.PurgeStream("REQUEST_STREAM")
		js.PurgeStream("DECISION_STREAM")

		err := js.DeleteKeyValue("SHIELDMESH_ENGINES")
		if err != nil {
			fmt.Printf("Failed to delete KV bucket: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"cleared"}`)
	})

	fmt.Println("Viewer API running on http://localhost:8000")
	fmt.Println("Endpoints:")
	fmt.Println("  - http://localhost:8000/requests")
	fmt.Println("  - http://localhost:8000/decisions")
	fmt.Println("  - http://localhost:8000/engines")
	fmt.Println("  - http://localhost:8000/clear (POST or GET to clear all)")

	log.Fatal(http.ListenAndServe(":8000", nil))
}
