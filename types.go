package main

import (
	"time"

	"github.com/tidwall/buntdb"
	"github.com/zmb3/spotify/v2"
	"golang.org/x/net/context"
	"gorm.io/gorm"
)

type Items struct {
	gorm.Model
	ItemID   int      `json:"itemID" gorm:"primarykey;not null;index"`
	Title    string   `json:"title"`
	Album    string   `json:"album"`
	Artist   string   `json:"artist"`
	Url      string   `json:"url"`
	ACRID    string   `json:"acrID"`
	Metadata Metadata `gorm:"embedded"`
	Comments Comments `json:"Comment" gorm:"foreignKey:ItemID;references:ItemID"`
}

type Comments struct {
	CommentID int `json:"commentID" gorm:"primarykey;not null;"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	Up        int            `json:"up" gorm:"not null;"`
	Down      int            `json:"down" gorm:"not null;"`
	Content   string         `json:"content" gorm:"not null;"`
	Created   *time.Time     `json:"created" gorm:"not null;"`
	ItemID    int            `json:"itemID" gorm:"not null;index"`
	Thumb     string         `json:"thumb" gorm:"not null;"`
}

type Metadata struct {
	DeezerURL     string `json:"deezerURL"`
	DeezerID      string `json:"deezerID" `
	SoundcloudURL string `json:"soundcloudURL"`
	SoundcloudID  string `json:"soundcloudID"`
	SpotifyURL    string `json:"spotifyURL"`
	SpotifyID     string `json:"spotifyID"`
	YoutubeURL    string `json:"youtubeURL" `
	YoutubeID     string `json:"youtubeID"`
	TidalURL      string `json:"tidalURL"`
	TidalID       string `json:"tidalID"`
	ApplemusicURL string `json:"applemusicURL"`
	ApplemusicID  string `json:"applemusicID"`
}

type fetcher struct {
	db      *gorm.DB
	spotify *spotify.Client
	ctx     context.Context
	bunt    *buntdb.DB
}
