package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"

	"encoding/csv"
	"html/template"

	"github.com/joho/godotenv"

	_ "github.com/lib/pq"
)

const (
	authorizeURL   = "https://github.com/login/oauth/authorize"
	accessTokenURL = "https://github.com/login/oauth/access_token"
	apiURL         = "https://api.github.com/user"
	reposURL       = "https://api.github.com/user/repos"
	emailURL       = "https://api.github.com/user/emails"
)

var (
	clientID     string
	clientSecret string
	redirectURI  = "http://localhost:8080/callback"
	stateStore   = sync.Map{}
	db           *sql.DB
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	clientID = os.Getenv("GITHUB_CLIENT_ID")
	clientSecret = os.Getenv("GITHUB_CLIENT_SECRET")

	dbUsername := os.Getenv("DB_USERNAME")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	connStr := fmt.Sprintf("user=%s password=%s dbname=%s sslmode=disable", dbUsername, dbPassword, dbName)
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Error opening database connection: %v", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatalf("Error pinging database: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS owners (
		id BIGINT PRIMARY KEY,
		login VARCHAR(255) NOT NULL,
		email VARCHAR(255)
	)`)

	if err != nil {
		log.Fatalf("Error creating owners table: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS repositories (
		id SERIAL PRIMARY KEY,
		repo_id BIGINT NOT NULL UNIQUE,
		name VARCHAR(255) NOT NULL,
		owner_id BIGINT NOT NULL,
		private BOOLEAN NOT NULL,
		stars_count INTEGER NOT NULL,
		FOREIGN KEY (owner_id) REFERENCES owners(id)
	)`)

	if err != nil {
		log.Fatalf("Error creating repositories table: %v", err)
	}

}

func main() {

	// Set up the logger
	logFile, err := setupLogger()
	if err != nil {
		log.Fatalf("Error setting up logger: %v", err)
	}
	defer logFile.Close()

	defer db.Close()

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/authorize", handleAuthorize)
	http.HandleFunc("/callback", handleCallback)
	http.HandleFunc("/download", handleDownload)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("index.html")
	if err != nil {
		log.Printf("Error parsing index.html: %v", err)
		http.Error(w, "Error parsing index.html", http.StatusInternalServerError)
		return
	}

	err = t.Execute(w, nil)
	if err != nil {
		log.Printf("Error executing index.html: %v", err)
		http.Error(w, "Error executing index.html", http.StatusInternalServerError)
		return
	}
}

func handleAuthorize(w http.ResponseWriter, r *http.Request) {
	state := generateRandomState()
	stateStore.Store(state, true)

	params := url.Values{}
	params.Add("client_id", clientID)
	params.Add("redirect_uri", redirectURI)
	params.Add("scope", "repo read:org user:email")
	params.Add("state", state)

	authURL := fmt.Sprintf("%s?%s", authorizeURL, params.Encode())
	http.Redirect(w, r, authURL, http.StatusFound)
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`SELECT o.id, o.login, COALESCE(o.email, ''), r.repo_id, r.name, r.private, r.stars_count
        FROM repositories r
        JOIN owners o ON r.owner_id = o.id`)
	if err != nil {
		log.Printf("Error querying repositories: %v", err)
		http.Error(w, "Error querying repositories", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment;filename=repositories.csv")

	cw := csv.NewWriter(w)
	cw.Write([]string{"Owner ID", "Owner Name", "Owner Email", "Repo ID", "Repo Name", "Status", "Stars Count"})
	for rows.Next() {
		var ownerID int64
		var ownerName, ownerEmail, repoName, status string
		var repoID, starsCount int64
		var private bool

		err = rows.Scan(&ownerID, &ownerName, &ownerEmail, &repoID, &repoName, &private, &starsCount)
		if err != nil {
			log.Printf("Error scanning repository row: %v", err)
			http.Error(w, "Error scanning repository row", http.StatusInternalServerError)
			return
		}

		if private {
			status = "Private"
		} else {
			status = "Public"
		}

		cw.Write([]string{strconv.FormatInt(ownerID, 10), ownerName, ownerEmail, strconv.FormatInt(repoID, 10), repoName, status, strconv.FormatInt(starsCount, 10)})
	}

	err = rows.Err()
	if err != nil {
		log.Printf("Error iterating through repository rows: %v", err)
		http.Error(w, "Error iterating through repository rows", http.StatusInternalServerError)
		return
	}

	cw.Flush()
}

func handleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	_, ok := stateStore.Load(state)
	if !ok {
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}
	stateStore.Delete(state)

	params := url.Values{}
	params.Add("client_id", clientID)
	params.Add("client_secret", clientSecret)
	params.Add("code", code)
	params.Add("redirect_uri", redirectURI)

	req, err := http.NewRequest("POST", accessTokenURL, strings.NewReader(params.Encode()))
	if err != nil {
		log.Printf("Error creating request to exchange code for token: %v", err)
		http.Error(w, "Error creating request to exchange code for token", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Error exchanging code for token: %v", err)
		http.Error(w, "Error exchanging code for token", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading access token response: %v", err)
		http.Error(w, "Error reading access token response", http.StatusInternalServerError)
		return
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		Scope       string `json:"scope"`
		TokenType   string `json:"token_type"`
	}

	err = json.Unmarshal(body, &tokenResp)
	if err != nil {
		log.Printf("Error unmarshalling access token response: %v", err)
		http.Error(w, "Error unmarshalling access token response", http.StatusInternalServerError)
		return
	}

	repos, err := fetchRepos(tokenResp.AccessToken)
	if err != nil {
		log.Printf("Error fetching repositories: %v", err)
		http.Error(w, "Error fetching repositories", http.StatusInternalServerError)
		return
	}

	err = saveRepos(repos)
	if err != nil {
		log.Printf("Error saving repositories: %v", err)
		http.Error(w, "Error saving repositories", http.StatusInternalServerError)
		return
	}

	// fmt.Fprintf(w, "Repositories saved successfully")
	http.Redirect(w, r, "/?authorized=true", http.StatusFound)
}

func fetchEmails(accessToken string) ([]string, error) {
	client := &http.Client{}

	req, err := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	err = json.Unmarshal(body, &emails)
	if err != nil {
		return nil, err
	}

	var userEmails []string
	for _, email := range emails {
		if email.Primary && email.Verified {
			userEmails = append(userEmails, email.Email)
			log.Printf("Email fetched: ", email.Email)
		}
	}

	return userEmails, nil
}

func fetchRepos(accessToken string) ([]map[string]interface{}, error) {
	client := &http.Client{}
	var allRepos []map[string]interface{}

	nextPage := 1
	for {
		req, err := http.NewRequest("GET", fmt.Sprintf("%s?visibility=all&page=%d&per_page=100&access_token=%s", reposURL, nextPage, accessToken), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
		req.Header.Set("Accept", "application/vnd.github+json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var repos []map[string]interface{}
		err = json.Unmarshal(body, &repos)
		if err != nil {
			return nil, err
		}

		if len(repos) == 0 {
			break
		}

		allRepos = append(allRepos, repos...)
		nextPage++
	}

	userEmails, err := fetchEmails(accessToken)
	if err != nil {
		return nil, err
	}

	for i := range allRepos {
		allRepos[i]["email"] = userEmails
	}

	return allRepos, nil
}

func saveRepos(repos []map[string]interface{}) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	ownerStmt, err := tx.Prepare(`INSERT INTO owners (id, login, email)
        VALUES ($1, $2, $3)
        ON CONFLICT (id) DO UPDATE SET login = $2, email = $3`)
	if err != nil {
		return err
	}
	defer ownerStmt.Close()

	repoStmt, err := tx.Prepare(`INSERT INTO repositories (repo_id, name, owner_id, private, stars_count)
        VALUES ($1, $2, $3, $4, $5)
        ON CONFLICT (repo_id) DO UPDATE SET name = $2, owner_id = $3, private = $4, stars_count = $5`)
	if err != nil {
		return err
	}
	defer repoStmt.Close()

	for _, repo := range repos {
		name, ok := repo["name"].(string)
		if !ok {
			log.Fatalf("Failed to convert name to string: %v", repo["name"])
		}
		log.Printf("Processing repo with name: %s", name)
		owner := repo["owner"].(map[string]interface{})
		_, err := ownerStmt.Exec(owner["id"], owner["login"], owner["email"])
		if err != nil {
			tx.Rollback()
			return err
		}

		_, err = repoStmt.Exec(repo["id"], repo["name"], owner["id"], repo["private"], repo["stargazers_count"])
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

func generateRandomState() string {
	data := make([]byte, 32)
	_, err := rand.Read(data)
	if err != nil {
		log.Fatalf("Error generating random state: %v", err)
	}
	return base64.StdEncoding.EncodeToString(data)
}
