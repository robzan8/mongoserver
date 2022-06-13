package main

import (
	"context"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	tableHtml string
	data      *mongo.Collection
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		log.Fatal("$PORT not set")
	}

	table, err := os.ReadFile("server/table.html")
	if err != nil {
		log.Fatalf("Error reading table file: %s", err)
	}
	tableHtml = string(table)

	opt := options.Client().ApplyURI("mongodb://localhost:27017")
	client, err := mongo.Connect(context.TODO(), opt)
	if err != nil {
		log.Fatalf("Error connecting to mongo: %s", err)
	}
	defer client.Disconnect(context.TODO())

	data = client.Database("test").Collection("data")

	http.Handle("/", http.FileServer(http.Dir("./server/static")))
	http.HandleFunc("/store", storeEndpoint)
	http.HandleFunc("/table", tableEndpoint)

	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func setAllowOrigins(h http.Header) { h.Set("Access-Control-Allow-Origin", "*") }

func storeEndpoint(w http.ResponseWriter, req *http.Request) {
	setAllowOrigins(w.Header())

	switch req.Method {
	case http.MethodOptions:
		// OK
	case http.MethodGet:
		fmt.Fprintln(w, "You should POST your json file here")
	case http.MethodPost:
		storePost(w, req)
	default:
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Unsupported method %s", req.Method)
	}
}

func storePost(w http.ResponseWriter, req *http.Request) {
	var err error // beware of shadowing
	defer func() {
		if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			fmt.Fprintf(w, "%s", err)
		}
	}()

	f, _, err := req.FormFile("data")
	if err != nil {
		return
	}
	defer f.Close()
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return
	}

	var doc interface{}
	err = bson.UnmarshalExtJSON(b, false, &doc)
	if err != nil {
		return
	}
	res, err := data.InsertOne(context.TODO(), doc)
	if err != nil {
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, "Document %s saved!", res.InsertedID.(primitive.ObjectID).Hex())
}

func tableEndpoint(w http.ResponseWriter, req *http.Request) {
	setAllowOrigins(w.Header())

	switch req.Method {
	case http.MethodOptions:
		// OK
	case http.MethodGet:
		tableGet(w, req)
	default:
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Unsupported method %s", req.Method)
	}
}

type Measurement struct {
	Time   string  `bson:"time"`
	Lat    float64 `bson:"latitude"`
	Lon    float64 `bson:"longitude"`
	Temp   float64 `bson:"temperature"`
	Hum    float64 `bson:"humidity"`
	Bright float64 `bson:"brightness"`
}

func tableGet(w http.ResponseWriter, req *http.Request) {
	var err error // beware of shadowing
	defer func() {
		if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			fmt.Fprintf(w, "%s", err)
		}
	}()

	cursor, err := data.Find(context.TODO(), bson.D{})
	if err != nil {
		return
	}
	defer cursor.Close(context.TODO())

	var docs []Measurement
	for cursor.Next(context.TODO()) {
		var doc Measurement
		err = cursor.Decode(&doc)
		if err != nil {
			return
		}
		docs = append(docs, doc)
	}

	t, err := template.New("table").Parse(tableHtml)
	if err != nil {
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t.Execute(w, docs)
}
