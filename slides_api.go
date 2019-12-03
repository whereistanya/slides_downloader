package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/slides/v1"
)

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func grabImage(client *http.Client, imageUrl string, fileName string) error {
	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	resp, err := client.Get(imageUrl)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	size, err := io.Copy(file, resp.Body)
	if err != nil {
		return err
	}
	if size == 0 {
		return fmt.Errorf("unexpected empty file from url %s", imageUrl)
	}

	return nil
}

func main() {
	notesFile := "notes.txt"
	// Add the presentation id. That's the url segment after
	// https://docs.google.com/presentation/d/ in the slide url.
	presentationId := "1MXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"

	b, err := ioutil.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, "https://www.googleapis.com/auth/presentations.readonly")
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	srv, err := slides.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve Slides client: %v", err)
	}

	pageSvc := slides.NewPresentationsPagesService(srv)

	httpClient := http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			r.URL.Opaque = r.URL.Path
			return nil
		},
	}
	notes, err := os.Create(notesFile)
	if err != nil {
		log.Fatalf("Couldn't create notes file at %s: %v", notesFile, err)
	}
	defer notes.Close()

	presentation, err := srv.Presentations.Get(presentationId).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve data from presentation: %v", err)
	}

	fmt.Printf("The presentation contains %d slides:\n", len(presentation.Slides))
	for i, page := range presentation.Slides {
		fmt.Printf("Slide %d:\n", i+1)
		notes.WriteString(fmt.Sprintf("Slide %d:\n", i+1))
		pageId := page.ObjectId

		thumbReq := pageSvc.GetThumbnail(presentationId, pageId)
		thumb, err := thumbReq.Do()
		if err != nil {
			log.Fatalf("Couldn't create thumbnail: %v", err)
		}

		err = grabImage(&httpClient, thumb.ContentUrl, fmt.Sprintf("image%d.jpg", i+1))
		if err != nil {
			log.Fatalf("Couldn't get image from %s: %v", thumb.ContentUrl, err)
		}

		for _, element := range page.SlideProperties.NotesPage.PageElements {
			text := element.Shape.Text
			if text != nil {
				for _, e := range element.Shape.Text.TextElements {
					run := e.TextRun
					if run != nil {
						notes.WriteString(fmt.Sprintf("%+v\n", run.Content))
					}
				}
			}
		}
	}
}
