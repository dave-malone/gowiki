package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"regexp"

	_ "github.com/go-sql-driver/mysql"
)

var (
	templates = template.Must(template.ParseFiles("tmpl/edit.html", "tmpl/view.html"))
	validPath = regexp.MustCompile("^/(edit|save|view)/([a-zA-Z0-9]+)$")
	addr      = flag.Bool("addr", false, "find open address and print to final-port.txt")
	dbUrl     = flag.String("dburl", "gowiki@localhost:3306/gowiki", "specify the MySQL database url to connect against")
)

type Page struct {
	Title string
	Body  []byte
}

type Person struct {
	Id       int    `json:"id,omitempty"`
	Name     string `json:name`
	Age      int32  `json:age`
	EyeColor string `json:eyeColor`
}

func (p *Person) save() (err error) {
	db := GetDb()
	_, err = db.Exec("INSERT INTO PERSON (NAME, AGE, EYE_COLOR) VALUES (?, ?, ?)", p.Name, p.Age, p.EyeColor)
	return
}

func (p *Page) save() error {
	filename := "data/" + p.Title + ".txt"
	return ioutil.WriteFile(filename, p.Body, 0600)
}

func GetDb() *sql.DB {
	db, err := sql.Open("mysql", *dbUrl)

	if err != nil {
		fmt.Printf("Failed to connect to db %s\n", dbUrl)
		panic(err.Error())
	}

	if err := db.Ping(); err == nil {
		fmt.Printf("Connected to db %s\n", *dbUrl)
	} else {
		fmt.Printf("Failed to connect to db %s\n", dbUrl)
		panic(err.Error())
	}

	return db
}

func InitDb() *sql.DB {
	db := GetDb()

	_, err := db.Exec(
		`CREATE TABLE PERSON (
			ID int NOT NULL AUTO_INCREMENT,
			NAME varchar(255) NOT NULL,
			AGE int,
			EYE_COLOR varchar(255),
			PRIMARY KEY (ID)
		) `,
	)

	if err != nil {
		fmt.Println("Failed to create the PERSON table:", err)
	}

	return db
}

func loadPage(title string) (*Page, error) {
	filename := "data/" + title + ".txt"
	body, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	return &Page{Title: title, Body: body}, nil
}

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		//Here we will extract the page title from the Request
		//and call the provided handler 'fn'
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}

		//The title is the second subexpression in the match.
		fn(w, r, m[2])
	}
}

func viewHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		http.Redirect(w, r, "/edit/"+title, http.StatusFound)
		return
	}
	renderTemplate(w, "view", p)
}

func editHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		p = &Page{Title: title}
	}

	renderTemplate(w, "edit", p)
}

func saveHandler(w http.ResponseWriter, r *http.Request, title string) {
	body := r.FormValue("body")
	p := &Page{Title: title, Body: []byte(body)}
	err := p.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/view/"+title, http.StatusFound)
}

func homePageHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/view/FrontPage", http.StatusFound)
}

func personHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method == "POST" {
		decoder := json.NewDecoder(req.Body)
		var person Person
		err := decoder.Decode(&person)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid Person request: %v", err), http.StatusInternalServerError)
		}

		log.Printf("Person: %+v", person)

		err = person.save()
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to save your person: %v", err), http.StatusInternalServerError)
		} else {
			fmt.Fprintf(w, "We saved %s for you", person.Name)
		}
	} else {
		http.Error(w, fmt.Sprintf("personHandler currently does not support %s requests", req.Method), http.StatusInternalServerError)
	}

}

func main() {
	flag.Parse()

	InitDb()

	http.HandleFunc("/view/", makeHandler(viewHandler))
	http.HandleFunc("/edit/", makeHandler(editHandler))
	http.HandleFunc("/save/", makeHandler(saveHandler))
	http.HandleFunc("/person/", personHandler)
	http.HandleFunc("/", homePageHandler)

	if *addr {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			log.Fatal(err)
		}

		err = ioutil.WriteFile("final-port.txt", []byte(l.Addr().String()), 0644)
		if err != nil {
			log.Fatal(err)
		}

		s := &http.Server{}
		s.Serve(l)
		return
	}

	http.ListenAndServe(":8080", nil)
}
