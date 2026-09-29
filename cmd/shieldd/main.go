package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/JohnnyAsh-U/shieldmesh"
	"github.com/JohnnyAsh-U/shieldmesh/shared"
	"github.com/JohnnyAsh-U/shieldmesh/transport"
	// "github.com/google/uuid"
)



func main(){
	ctx := context.Background()
	tr, err := transport.NewNatsTransport("localhost:4222")	
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("Connected")

	shieldMesh := shieldmesh.NewShieldMesh(shieldmesh.Config{
		Transport: tr,
		Name: "node-1",
		FailPolicy: shared.FAILCLOSED,
	})
	shieldMesh.Start(ctx)



	// shieldMesh.Check(ctx, shared.Event{})
	shieldMesh.Observe(ctx, shared.Request{
		ID: "2234",
		Method: "Get",
	})

	defer shieldMesh.Stop()


	fmt.Println("Observer")
	

	mux := http.NewServeMux()

	mux.HandleFunc("/debug/nats", func (w http.ResponseWriter, r *http.Request)  {
		// engines, _ := tr.ListEngines(ctx)
		dec, _ := tr.GetDecision(ctx, shared.Subject{ID: "4", Type: "ip"})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"engines": dec,
			"last_seq": 4,
			"bans_count": 0,
		})
	})

	mux.HandleFunc("/debug/bans", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(nil)
	}) 


	http.ListenAndServe(":9091", mux)
}