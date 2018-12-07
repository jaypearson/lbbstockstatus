package main

import (
	"fmt"
	"github.com/anaskhan96/soup"
	"os"
	"net/http"
	"net/url"
	"io/ioutil"
	"strings"
)

func main() {
	scrapeurl := "http://www.lbbncabc.com/views/em_abc_stockstatus.asp"
	resp, err := http.PostForm(scrapeurl, url.Values{"EMAccount": {"abc002"}, "EMPassword": {"1houjLEx"}}) 
	if err != nil {
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	doc := soup.HTMLParse(string(body))
	rows := doc.FindAll("tr")
	for _, row := range rows {
		data := row.FindAll("td")
		for _, d := range data {
			fmt.Print(strings.TrimSpace(d.Text()), "|")
		}
		fmt.Println()
	}
}

