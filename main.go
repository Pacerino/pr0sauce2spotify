package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/buntdb"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var (
	auth  = spotifyauth.New(spotifyauth.WithRedirectURL("http://localhost:8080/callback"), spotifyauth.WithScopes(spotifyauth.ScopeUserReadPrivate, spotifyauth.ScopePlaylistReadPrivate, spotifyauth.ScopePlaylistModifyPrivate))
	ch    = make(chan *spotify.Client)
	state = "abc123"
)

func main() {
	godotenv.Load()     // Load .env File
	fetch := &fetcher{} // Create a new fetcher instance
	fetch.initDB()      // Init Database
	fetch.initBunt()    // Init BuntDB
	fetch.initSpotify() // Init Spotify
	fetch.execute()     // Execute main code
}

func (u *fetcher) initDB() {
	var err error
	dataSourceName := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", os.Getenv("MYSQL_USER"), os.Getenv("MYSQL_PASSWORD"), os.Getenv("MYSQL_HOST"), os.Getenv("MYSQL_DATABASE"))
	u.db, err = gorm.Open(mysql.Open(dataSourceName))

	if err != nil {
		log.WithError(err).Fatal("Could not open connection to DB")
	}
}

func (u *fetcher) initBunt() {
	db, err := buntdb.Open("data.db")
	if err != nil {
		log.Fatal(err)
	}
	u.bunt = db
}

func (u *fetcher) initSpotify() {
	u.ctx = context.Background()

	tokenExists := u.tokenExists()

	if !tokenExists {
		u.handleSpotifyLogin() // Create new Login with new Tokens etc.
	} else {
		tok_access, err := u.readBunt("tok_access")
		if err != nil {
			if !strings.Contains(err.Error(), "not found") {
				log.Fatal(err)
			}
		}
		tok_type, err := u.readBunt("tok_type")
		if err != nil {
			if !strings.Contains(err.Error(), "not found") {
				log.Fatal(err)
			}
		}
		tok_refresh, err := u.readBunt("tok_refresh")
		if err != nil {
			if !strings.Contains(err.Error(), "not found") {
				log.Fatal(err)
			}
		}
		tok_expire, err := u.readBunt("tok_expire")
		if err != nil {
			if !strings.Contains(err.Error(), "not found") {
				log.Fatal(err)
			}
		}
		expireToken, _ := time.Parse(time.RFC822, tok_expire)
		token := &oauth2.Token{
			Expiry:       expireToken,
			TokenType:    tok_type,
			AccessToken:  tok_access,
			RefreshToken: tok_refresh,
		}
		httpClient := oauth2.NewClient(u.ctx, oauth2.StaticTokenSource(token))
		client := spotify.New(httpClient)
		u.spotify = client

		user, err := client.CurrentUser(u.ctx)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Logged in as %s", user.ID)
	}
}

func (u *fetcher) handleSpotifyLogin() {
	http.HandleFunc("/callback", u.completeAuth)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Got request for:", r.URL.String())
	})
	go func() {
		err := http.ListenAndServe(":8080", nil)
		if err != nil {
			log.Fatal(err)
		}
	}()

	url := auth.AuthURL(state)
	log.Println("Please login: %s", url)

	client := <-ch
	user, err := client.CurrentUser(u.ctx)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Logged in as %s", user.ID)
}

func (u *fetcher) execute() {
	playlist, err := u.spotify.GetPlaylist(u.ctx, spotify.ID(os.Getenv("SPOTIFY_PLAYLIST")))
	if err != nil {
		log.Fatalln(err)
	}
	rows, err := u.db.Model(&Items{}).Select("items.spotify_id, comments.up - comments.down AS benis").Joins("LEFT JOIN comments ON items.item_id = comments.item_id").Where("CHAR_LENGTH(spotify_id) > 5").Order("benis DESC").Debug().Rows()
	if err != nil {
		log.Fatalln(err)
	}
	defer rows.Close()
	var item Items
	for rows.Next() {
		if err := u.db.ScanRows(rows, &item); err != nil {
			log.Fatalln(err)
		}
		spotifyID := item.Metadata.SpotifyID
		s := strings.Split(spotifyID, "::")
		id := s[1]
		snapshotID, err := u.spotify.AddTracksToPlaylist(u.ctx, spotify.ID(os.Getenv("SPOTIFY_PLAYLIST")), spotify.ID(id))
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Added %s to %s with SnapshotID %s", id, playlist.Name, snapshotID)
	}
}

func (u *fetcher) completeAuth(w http.ResponseWriter, r *http.Request) {
	tok, err := auth.Token(r.Context(), state, r)
	if err != nil {
		http.Error(w, "Couldn't get token", http.StatusForbidden)
		log.Fatal(err)
	}
	if st := r.FormValue("state"); st != state {
		http.NotFound(w, r)
		log.Fatalf("State mismatch: %s != %s\n", st, state)
	}

	client := spotify.New(auth.Client(r.Context(), tok))
	fmt.Fprintf(w, "Login Completed!")
	u.writeBunt("tok_type", tok.TokenType)
	u.writeBunt("tok_access", tok.AccessToken)
	u.writeBunt("tok_expire", tok.Expiry.Format(time.RFC822))
	u.writeBunt("tok_refresh", tok.RefreshToken)
	u.spotify = client
	ch <- client
}

func (u *fetcher) writeBunt(key string, value string) (err error) {
	err = u.bunt.Update(func(tx *buntdb.Tx) error {
		_, _, err := tx.Set(key, value, nil)
		return err
	})
	return err
}

func (u *fetcher) readBunt(key string) (value string, err error) {
	err = u.bunt.View(func(tx *buntdb.Tx) error {
		val, err := tx.Get(key)
		if err != nil {
			return err
		}
		value = val
		return nil
	})
	return value, err
}

func (u *fetcher) tokenExists() bool {
	tok_access, err := u.readBunt("tok_access")
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return false
		}
		log.Fatal(err)
	}
	return len(tok_access) > 3
}
