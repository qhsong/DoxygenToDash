package main

import (
	"database/sql"
	"fmt"
	"github.com/DeanThompson/syncmap"
	"github.com/PuerkitoBio/goquery"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"os"
	"strings"
	"time"
)

var UrlMap *syncmap.SyncMap
var db *sql.DB
var flog *log.Logger

func addEntryType(typename string, s *goquery.Selection) {
	linkLabel := s.Find(".memItemRight").Find("a")
	linkLabel.Each(func(i int, s *goquery.Selection) {
		structName := s.Text()
		link, _ := s.Attr("href")
		link = strings.Trim(link, "\r\n ")
		_, err := db.Exec("insert or ignore into searchIndex(name,type,path) VALUES('" + structName + "','" + typename + "','" + link + "')")
		if err != nil {
			log.Fatal("Insert " + typename + " " + structName + "Failed!")
			return
		}
		log.Print("Insert " + typename + structName)
	})
}
func addFunAndEnum(typeName string, s *goquery.Selection) {
	linkLabel := s.Find(".memItemRight")
	linkLabel.Each(func(i int, s *goquery.Selection) {
		funlabel := s.Find("a").First()
		funName := funlabel.Text()
		funAddr, _ := funlabel.Attr("href")
		_, err := db.Exec("insert or ignore into searchIndex(name,type,path) VALUES('" + funName + "','" + typeName + "','" + funAddr + "')")
		if err != nil {
			log.Fatal("Insert " + typeName + " " + funName + "Failed!")
			return
		}
		log.Print("Insert " + typeName + " " + funName)

	})
}

func main() {
	if len(os.Args) == 4 {
		logFile, _ := os.OpenFile("file.log", os.O_CREATE|os.O_RDWR, 0666)
		defer logFile.Close()
		log.SetOutput(logFile)
		file, err := os.Open(os.Args[1] + os.Args[2])
		if err != nil {
			log.Fatal("Unable to open" + os.Args[1])
			return
		}
		defer file.Close()

		//conn the SQLite3
		db, err = sql.Open("sqlite3", os.Args[3])
		if err != nil {
			log.Fatal("Cann't connect to SQLite3")
			return
		}
		db.Exec("CREATE TABLE searchIndex(id INTEGER PRIMARY KEY, name TEXT, type TEXT, path TEXT);")
		db.Exec("CREATE UNIQUE INDEX anchor ON searchIndex (name, type, path);")
		defer db.Close()

		//Parse the html file
		doc, err := goquery.NewDocumentFromReader(file)
		if err != nil {
			log.Fatal("Unable to goquery this file")
			return
		}

		UrlChan := make(chan string, 200)
		ExitFlag := make(chan bool)
		defer close(UrlChan)
		defer close(ExitFlag)

		UrlMap = syncmap.New()
		UrlMap.Set(os.Args[2], 1)
		UrlMap.Set("http://www.doxygen.org/index.html", 1)

		go parseFile(&UrlChan, ExitFlag)
		fmt.Println(&UrlChan)

		baselink := doc.Find("a")
		baselink.Each(func(i int, s *goquery.Selection) {
			link, _ := s.Attr("href")
			keyword := s.Text()
			keyword = strings.Trim(keyword, "\r\n ")
			if _, ok := UrlMap.Get(keyword); ok == false {
				_, err = db.Exec("insert or ignore into searchIndex(name,type,path) VALUES('" + keyword + "','Word','" + link + "')")
				if err != nil {
					log.Fatal(err)
					return
				}
				log.Print(link)
				UrlChan <- link
			}
		})
		<-ExitFlag
	} else {
		fmt.Println("USAGE:./GeneratorIndex DirName indexfile sql3file")
	}

}

func parseFile(UrlList *chan string, ExitFlag chan bool) {
	fmt.Println("ParseFile Launched!")
	for true {
		select {
		case <-time.After(time.Second * 3):
			log.Print("Routine Exited")
			ExitFlag <- true
			return
		case url := <-*UrlList:
			if _, ok := UrlMap.Get(url); ok == false { //Not parse
				log.Print("Add " + url)
				UrlMap.Set(url, 1)

				//add file into UrlList
				file, _ := os.Open(os.Args[1] + url)
				doc, _ := goquery.NewDocumentFromReader(file)
				filelist := doc.Find(".textblock").First().Find("code").Find("a")
				filelist.Each(func(i int, s *goquery.Selection) {
					filename := s.Text()
					newUrl, _ := s.Attr("href")
					if _, ok := UrlMap.Get(filename); ok == false {
						_, err := db.Exec("insert or ignore into searchIndex(name,type,path) VALUES('" + filename + "','File','" + newUrl + "')")
						if err != nil {
							log.Fatal(err)
						}
						*UrlList <- newUrl
					}
				})
				otherList := doc.Find(".memberdecls")
				otherList.Each(func(i int, s *goquery.Selection) {
					name := s.Find(".groupheader").Text()
					name = strings.Trim(name, "\r\n ")
					switch name {
					case "Data Structures":
						addEntryType("Struct", s)
					case "Macros":
						addEntryType("Macro", s)
					case "Typedefs":
						addEntryType("Define", s)
					case "Functions":
						addFunAndEnum("Function", s)
					case "Enumerations":
						addFunAndEnum("Enum", s)
					case "Variables":
						addEntryType("Variable", s)
					default:
						log.Fatal("Unknow type " + name + " in " + os.Args[1] + url)
					}
				})
			}
		}
	}
}
