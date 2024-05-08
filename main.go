package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/machinebox/graphql"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/sheets/v4"
)

func getGitHubPRComments(client *graphql.Client, token, owner, repoName string) ([][]interface{}, error) {
    // GraphQL query
    query := fmt.Sprintf(`
        query {
            repository(owner: "%s", name: "%s") {
                pullRequests(first: 10, states: MERGED) {
                    edges {
                        node {
                            number
                            title
                            body
                            author {
                                login
                            }
                            comments(first: 5) {
                                edges {
                                    node {
                                        body
                                        author {
                                            login
                                        }
                                        createdAt
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }
    `, owner, repoName)

    // Prepare request
    req := graphql.NewRequest(query)
    req.Header.Set("Authorization", "Bearer "+token)

    // Execute request
    var result struct {
        Repository struct {
            PullRequests struct {
                Edges []struct {
                    Node struct {
                        Number   int
                        Title    string
                        Body     string
                        Author   struct {
                            Login string
                        }
                        Comments struct {
                            Edges []struct {
                                Node struct {
                                    Body      string
                                    Author    struct {
                                        Login string
                                    }
                                    CreatedAt string
                                }
                            }
                        }
                    }
                }
            }
        }
    }

    if err := client.Run(context.Background(), req, &result); err != nil {
        return nil, err
    }

    // Convert to table data
    rows := [][]interface{}{
        {"Number", "Title", "Author", "Comment Author", "Comment Body", "Comment Created At"},
    }
    for _, pr := range result.Repository.PullRequests.Edges {
        for _, comment := range pr.Node.Comments.Edges {
            rows = append(rows, []interface{}{
                pr.Node.Number, pr.Node.Title, pr.Node.Author.Login,
                comment.Node.Author.Login, comment.Node.Body, comment.Node.CreatedAt,
            })
        }
    }
    return rows, nil
}

func writeToGoogleSheets(spreadsheetID string, rows [][]interface{}) error {
    // Read credentials
    credentials, err := os.ReadFile("credentials.json")
    if err != nil {
        return err
    }

    // Create Sheets client
    config, err := google.JWTConfigFromJSON(credentials, sheets.SpreadsheetsScope)
    if err != nil {
        return err
    }
    client := config.Client(context.Background())
    service, err := sheets.New(client)
    if err != nil {
        return err
    }

    // Write data
    rangeData := "Sheet1!A1"
    valueRange := &sheets.ValueRange{
        Values: rows,
    }
    _, err = service.Spreadsheets.Values.Update(spreadsheetID, rangeData, valueRange).ValueInputOption("RAW").Do()
    return err
}

func main() {
    // Load .env file
    godotenv.Load()
    githubToken := os.Getenv("GITHUB_TOKEN")
    owner := os.Getenv("OWNER")
    repoName := os.Getenv("REPO_NAME")
    spreadsheetID := os.Getenv("SPREADSHEET_ID")

    // Initialize GraphQL client
    client := graphql.NewClient("https://api.github.com/graphql")

    // Get GitHub PR comments
    rows, err := getGitHubPRComments(client, githubToken, owner, repoName)
    if err != nil {
        log.Fatalf("Error getting GitHub PR comments: %v", err)
    }

    // Write data to Google Sheets
    if err := writeToGoogleSheets(spreadsheetID, rows); err != nil {
        log.Fatalf("Error writing to Google Sheets: %v", err)
    }
    fmt.Println("Successfully wrote to Google Sheets")
}
