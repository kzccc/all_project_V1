package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"echo_chat_server/internal/auth"
	"echo_chat_server/internal/dao"
	"echo_chat_server/internal/model"
)

type tokenRecord struct {
	UUID        string `json:"uuid"`
	Telephone   string `json:"telephone"`
	AccessToken string `json:"access_token"`
}

func main() {
	var (
		prefix string
		count  int
		output string
	)
	flag.StringVar(&prefix, "prefix", "WS", "business user prefix without leading U")
	flag.IntVar(&count, "count", 1000, "number of tokens to generate")
	flag.StringVar(&output, "output", "", "output file path")
	flag.Parse()

	prefix = strings.ToUpper(strings.TrimSpace(prefix))
	if prefix == "" {
		fmt.Fprintln(os.Stderr, "prefix cannot be empty")
		os.Exit(1)
	}
	if count <= 0 {
		fmt.Fprintln(os.Stderr, "count must be positive")
		os.Exit(1)
	}
	if strings.TrimSpace(output) == "" {
		fmt.Fprintln(os.Stderr, "output cannot be empty")
		os.Exit(1)
	}

	var users []model.UserInfo
	if err := dao.GormDB.
		Where("uuid LIKE ?", "U"+prefix+"%").
		Order("telephone ASC").
		Limit(count).
		Find(&users).Error; err != nil {
		fmt.Fprintf(os.Stderr, "load users failed: %v\n", err)
		os.Exit(1)
	}
	if len(users) < count {
		fmt.Fprintf(os.Stderr, "insufficient users: need %d, found %d\n", count, len(users))
		os.Exit(1)
	}

	records := make([]tokenRecord, 0, len(users))
	for i, user := range users {
		sessionID := fmt.Sprintf("TSBENCH%011d", i+1)
		accessToken, err := auth.GenerateAccessToken(user, sessionID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "generate token failed for %s: %v\n", user.Uuid, err)
			os.Exit(1)
		}
		records = append(records, tokenRecord{
			UUID:        user.Uuid,
			Telephone:   user.Telephone,
			AccessToken: accessToken,
		})
	}

	content, err := json.Marshal(records)
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal token file failed: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(output, content, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write token file failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("generated %d access tokens into %s\n", len(records), output)
}
