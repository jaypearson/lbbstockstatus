package main

import (
	_ "database/sql"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/anaskhan96/soup"

	_ "github.com/denisenkom/go-mssqldb"
	"github.com/jmoiron/sqlx"
)

var sqlHostname string
var sqlPort int
var sqlUsername string
var sqlPassword string
var sqlInstance string
var sqlDBName string
var lbbUsername string
var lbbPassword string

var schema = `
IF (NOT EXISTS (SELECT * 
                 FROM INFORMATION_SCHEMA.TABLES 
                 WHERE TABLE_SCHEMA = 'dbo' 
                 AND  TABLE_NAME = 'LBBStockStatus'))
BEGIN
    CREATE TABLE LBBStockStatus (
		Code nvarchar(20),
		Company text,
		Brand text,
		Balance int,
		Comments text,
		Allocation int,
		Size text,
		CasesPerPallet int,
		ImportDate datetime DEFAULT GETDATE()
	);
END
IF (EXISTS (SELECT * 
                 FROM INFORMATION_SCHEMA.TABLES 
                 WHERE TABLE_SCHEMA = 'dbo' 
                 AND  TABLE_NAME = 'LBBStockStatusTemp'))
BEGIN
	DROP TABLE LBBStockStatusTemp;
END
CREATE TABLE LBBStockStatusTemp (
	Code nvarchar(20),
	Company text,
	Brand text,
	Balance int,
	Comments text,
	Allocation int,
	Size text,
	CasesPerPallet int,
	ImportDate datetime default GETDATE()
);
`

var mergeSQL = `
MERGE INTO dbo.LBBStockStatus as dest
	USING (SELECT * FROM dbo.LBBStockStatusTemp) as source
	ON dest.Code = source.Code
	WHEN MATCHED THEN
		UPDATE SET
			dest.Code = source.Code,
			dest.Company= source.Company,
			dest.Brand = source.Brand,
			dest.Balance = source.Balance,
			dest.Comments = source.Comments,
			dest.Allocation = source.Allocation,
			dest.Size = source.Size,
			dest.CasesPerPallet = source.CasesPerPallet,
			dest.ImportDate = source.ImportDate
	WHEN NOT MATCHED THEN
		INSERT VALUES (source.Code, source.Company, source.Brand, source.Balance, source.Comments,
			source.Allocation, source.Size, source.CasesPerPallet, source.ImportDate);`

type LBBStockStatus struct {
	Code           string `db:"Code"`
	Company        string `db:"Company"`
	Brand          string `db:"Brand"`
	Balance        int    `db:"Balance"`
	Comments       string `db:"Comments"`
	Allocation     int    `db:"Allocation"`
	Size           string `db:"Size"`
	CasesPerPallet int    `db:"CasesPerPallet"`
}

func init() {
	flag.StringVar(&sqlHostname, "hostname", "127.0.0.1", "The hostname or IP address of the SQL Server")
	flag.IntVar(&sqlPort, "port", 1433, "The port number of the SQL Server")
	flag.StringVar(&sqlUsername, "username", "", "Username for SQL Server")
	flag.StringVar(&sqlPassword, "password", "", "Password for SQL Server")
	flag.StringVar(&sqlInstance, "instance", "SQLExpress", "Instance name for SQL Server")
	flag.StringVar(&sqlDBName, "dbName", "RetailPOS", "Database to use in SQL Server")
	flag.StringVar(&lbbUsername, "lbbUsername", "", "Username for LB&B")
	flag.StringVar(&lbbPassword, "lbbPassword", "", "Password for LB&B")
}

func checkFlags() bool {
	var result = true
	var fields strings.Builder

	if lbbUsername == "" {
		fields.WriteString("lbbUsername ")
		result = false
	}
	if lbbPassword == "" {
		fields.WriteString("lbbPassword ")
		result = false
	}
	if sqlUsername == "" {
		fields.WriteString("sqlUsername ")
		result = false
	}
	if sqlPassword == "" {
		fields.WriteString("sqlPassword ")
		result = false
	}
	if fields.Len() > 0 {
		fmt.Println("Missing required fields: ", fields.String())
	}
	return result
}

func main() {
	fmt.Println("LB&B Stock Status Web Scraper starting up...")
	flag.Parse()
	if !checkFlags() {
		flag.PrintDefaults()
		os.Exit(1)
	}

	scrapeurl := "http://www.lbbncabc.com/views/em_abc_stockstatus.asp"
	fmt.Println("Downloading from ", scrapeurl)
	resp, err := http.PostForm(scrapeurl, url.Values{"EMAccount": {lbbUsername}, "EMPassword": {lbbPassword}})
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println("Parsing results")
	doc := soup.HTMLParse(string(body))

	query := url.Values{}
	query.Add("app name", "LBBStockStatus")
	query.Add("connection timeout", "30")
	if sqlDBName != "" {
		query.Add("database", sqlDBName)
	}

	u := &url.URL{
		Scheme:   "sqlserver",
		User:     url.UserPassword(sqlUsername, sqlPassword),
		Host:     fmt.Sprintf("%s:%d", sqlHostname, sqlPort),
		Path:     sqlInstance,
		RawQuery: query.Encode(),
	}
	fmt.Println("Connecting to SQL Server:\n\t", u.String())
	db, err := sqlx.Connect("sqlserver", u.String())
	if err != nil {
		log.Fatalln(err)
	}

	db.MustExec(schema)

	rows := doc.FindAll("tr")
	count := 0
	insertSQL := "INSERT INTO LBBStockStatusTemp (Code, Company, Brand, Balance, Comments, Allocation, Size, CasesPerPallet) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"
	for _, row := range rows {
		data := row.FindAll("td")
		if len(data) < 8 ||
			strings.TrimSpace(data[0].Text()) == "NC CODE" {
			continue
		}
		count++
		db.MustExec(db.Rebind(insertSQL),
			strings.TrimSpace(data[0].Text()),
			strings.TrimSpace(data[1].Text()),
			strings.TrimSpace(data[2].Text()),
			strings.TrimSpace(data[3].Text()),
			strings.TrimSpace(data[4].Text()),
			strings.TrimSpace(data[5].Text()),
			strings.TrimSpace(data[6].Text()),
			strings.TrimSpace(data[7].Text()))
		/* for _, d := range data {
			fmt.Print(strings.TrimSpace(d.Text()), "|")
		}
		fmt.Println()*/
	}
	db.MustExec(mergeSQL)
	fmt.Println(count, " records imported")
}
