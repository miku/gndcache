// gndcache server caching http://d-nb.info/gnd/{gnd}/about/rdf XML snippets
// in a local sqlite3 database, which can be prefilled with GNDCacheDB task
// from http://git.io/_mczZQ
package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"runtime/pprof"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
	"github.com/miku/gndcache"
)

// addNamespaces add
func addNamespaces(s string) string {
	namespaces := map[string]string{
		"bibo":     "http://purl.org/ontology/bibo/",
		"dc":       "http://purl.org/dc/elements/1.1/",
		"dcterms":  "http://purl.org/dc/terms/",
		"foaf":     "http://xmlns.com/foaf/0.1/",
		"gndo":     "http://d-nb.info/standards/elementset/gnd#",
		"isbd":     "http://iflastandards.info/ns/isbd/elements/",
		"lib":      "http://purl.org/library/",
		"marcRole": "http://id.loc.gov/vocabulary/relators/",
		"owl":      "http://www.w3.org/2002/07/owl#",
		"rda":      "http://rdvocab.info/",
		"rdf":      "http://www.w3.org/1999/02/22-rdf-syntax-ns#",
		"rdfs":     "http://www.w3.org/2000/01/rdf-schema#",
		"skos":     "http://www.w3.org/2004/02/skos/core#",
		"umbel":    "http://umbel.org/umbel#",
	}

	var buffer bytes.Buffer
	buffer.WriteString("<rdf:RDF\n")
	for k, v := range namespaces {
		buffer.WriteString(fmt.Sprintf("xmlns:%s=\"%s\"\n", k, v))
	}
	buffer.WriteString(">")
	buffer.WriteString(s)
	buffer.WriteString("\n</rdf:RDF>")
	return buffer.String()
}

func main() {
	path := flag.String("dbpath", "", "path to sqlite3 database")
	version := flag.Bool("v", false, "prints current program version")
	addr := flag.String("addr", ":5000", "host:port to listen on")
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to file")

	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	if *version {
		fmt.Println(gndcache.Version)
		os.Exit(0)
	}

	if *path == "" {
		log.Fatal("dbpath is required")
	}

	// get db
	db, err := sql.Open("sqlite3", *path)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// make sure we have a db to begin with
	s := `CREATE TABLE IF NOT EXISTS gnd (id text PRIMARY KEY,
					content blob,
					updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP)`
	_, err = db.Exec(s)
	if err != nil {
		log.Fatalf("%q: %s\n", err, s)
	}

	stmt, err := db.Prepare("select content from gnd where id = ?")
	defer stmt.Close()

	ins, err := db.Prepare("insert into gnd (id, content) values (?, ?)")
	defer ins.Close()

	r := mux.NewRouter()

	r.HandleFunc("/gnd/{gnd}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		http.Redirect(w, r, fmt.Sprintf("/cache/%s", vars["gnd"]), http.StatusSeeOther)
	})

	r.HandleFunc("/cache/{gnd}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		if r.Method == "GET" {
			var content string
			err = stmt.QueryRow(vars["gnd"]).Scan(&content)
			if err == nil {
				fmt.Fprint(w, addNamespaces(content))
				return
			}
			url := fmt.Sprintf("http://d-nb.info/gnd/%s/about/rdf", vars["gnd"])
			resp, err := http.Get(url)
			defer resp.Body.Close()
			b, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Fatal(err)
			}
			if resp.StatusCode != 200 {
				w.WriteHeader(resp.StatusCode)
				w.Write([]byte(fmt.Sprintf("%d %s\n", resp.StatusCode, http.StatusText(resp.StatusCode))))
			} else if err != nil {
				http.NotFound(w, r)
			} else {
				tx, err := db.Begin()
				if err != nil {
					log.Fatal(err)
				}
				if err != nil {
					log.Fatal(err)
				}
				_, err = ins.Exec(vars["gnd"], string(b))
				tx.Commit()
				fmt.Fprint(w, addNamespaces(string(b)))
			}
		}
	}).Methods("GET")

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		msg := fmt.Sprintf("Cached RDF/XML documents from GND. Example: http://%s/cache/118514768", r.Host)
		t := map[string]string{"msg": msg, "version": gndcache.Version}
		b, _ := json.Marshal(t)
		fmt.Fprintln(w, string(b))
	}).Methods("GET")

	http.Handle("/", r)
	log.Printf("db at %s, listening on %s\n", *path, *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
