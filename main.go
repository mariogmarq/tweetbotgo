package main

import (
    "math/rand"
	"database/sql"
	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"os"
	"strconv"
    "time"
)

func main() {
	//prepares connection to DB
	if !checkDB() {
		log.Fatal("Data base doesn't exists")
	}
	//Connects to db
	sqlDB, err := sql.Open("sqlite3", "../db/database.db")
	if err != nil {
		log.Fatal(err.Error())
	}
	defer sqlDB.Close() //Defer closing de connection

	//Preparation for twitter access
	config := oauth1.NewConfig("clave generica", "clave generica")
	token := oauth1.NewToken("clave generica", "clave generica")
	httpClient := config.Client(oauth1.NoContext, token)
	//The twitter client
	client := twitter.NewClient(httpClient)

    //Checks every user in the users table and execute routine over them
    rows, err := sqlDB.Query("SELECT COUNT(*) FROM users")
    if err != nil {
        log.Fatal(err.Error())
    }
    count := checkCount(rows)
    rows.Close();

    for {
        //update responses
        newResponses(sqlDB, client)
        for i := 0; i < count; i++ {
            routine(i, client, sqlDB)
        }
        time.Sleep(60*5*time.Second)
    }
}

//checkDB ensures that the database exists
func checkDB() bool {
	if _, err := os.Stat("../db/database.db"); err == nil {
		return true
	} else {
		return false
	}
}

//routine checks the last status of the user and answer them. It will also update is last tweet in case is not the one registered in the DB
func routine(i int, client *twitter.Client, db *sql.DB) {
    //Make a query for the user
    query := "SELECT * FROM users WHERE num="+strconv.Itoa(i)
    row:= db.QueryRow(query+";")

    //We read from the query the user data
    var num int
	var iD int64
	var screenname string
	var lastTweet string
	row.Scan(&num, &iD, &screenname, &lastTweet)
	user, _, err := client.Users.Show(&twitter.UserShowParams{UserID: iD})
	if err != nil {
		log.Fatalln(err.Error())
	}

    //If the status isn't the last saved then we will actualize the DB and
    //respond the tweet
	if user.Status.IDStr != lastTweet && !user.Status.Retweeted {
        log.Println("Updating and posting")
        //Update the last tweet
		Update := "Update users SET lasttweet = " + user.Status.IDStr + " WHERE num= " + strconv.Itoa(num)
		statement, err := db.Prepare(Update)
		if err != nil {
			log.Println(err.Error())
			return
		}
		_, err = statement.Exec()
		if err != nil {
			log.Fatalln(err.Error())
        }

        //List the responses and respond with it
		rows, _ := db.Query("SELECT COUNT(*) FROM responses")
		count := checkCount(rows)
        rows.Close()

        //Select one randomly
        selected := rand.Intn(count)
        var num int
        var text string
        query := "SELECT * FROM responses WHERE id="+strconv.Itoa(selected)
        row = db.QueryRow(query)
        row.Scan(&num, &text)
        log.Println("text: ",text, " num: ", selected)

        //Response
        client.Statuses.Update(text + " " + screenname, &twitter.StatusUpdateParams{InReplyToStatusID: user.Status.ID})
        
	}
}

//Allow us to count the number of rows in a table, for further information please check is use in the main function of the program
func checkCount(rows *sql.Rows) (count int) {
	for rows.Next() {
		rows.Scan(&count)
	}
	return count
}

//Checks MD and inserts new responses eliminating the messages
func newResponses(db *sql.DB, client *twitter.Client) {
	//gets the last 20 messages
	messages, _, err := client.DirectMessages.EventsList(&twitter.DirectMessageEventsListParams{Count: 20})
	if err != nil {
		return
	}

	for _, v := range messages.Events {
		if v.Message.Data.Text[0:5] == "newR " {
			//inserts it into the DB

			//get the useful part of the text
			text := v.Message.Data.Text[5:len(v.Message.Data.Text)]
			//get the number of rows in responses
			rows, _ := db.Query("SELECT COUNT(*) FROM responses")
			defer rows.Close()
			count := checkCount(rows)
			//Get ready to insert
			update := `INSERT INTO responses (id, text) values(?, ?)`
			stm, _ := db.Prepare(update)
			stm.Exec(count, text)
		}
		//Finally the message gets deleted
		client.DirectMessages.EventsDestroy(v.ID)

	}
}
