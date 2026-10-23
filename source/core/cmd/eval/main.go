package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"docssearch/core/internal/eval"
	"docssearch/core/internal/texts"
)

type assistantLine struct {
	Kind      string `json:"kind"`
	Citations []struct {
		Paragraph string `json:"paragraph"`
	} `json:"citations"`
}

func main() {
	questions, err := eval.Load_Questions()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	apiURL := strings.TrimRight(os.Getenv("EVAL_API_URL"), "/")
	if apiURL == "" {
		apiURL = "http://127.0.0.1:8080"
	}
	client := &http.Client{Timeout: 3 * time.Minute}
	token, loginError := login_Employee(client, apiURL)
	if loginError != nil {
		fmt.Println(loginError.Error())
		return
	}
	pointQuestions := 0
	pointHits := 0
	fallbackCount := 0
	for _, question := range questions {
		reply, askError := ask_Question(client, apiURL, token, question.Question)
		if askError != nil {
			fmt.Printf("%s: %s\n", question.Question, askError.Error())
			if question.Paragraph != "" {
				pointQuestions++
			}
			continue
		}
		fmt.Printf("%s: %s\n", question.Question, reply.Kind)
		if reply.Kind == texts.KindFallback {
			fallbackCount++
		}
		if question.Paragraph == "" {
			continue
		}
		pointQuestions++
		for _, citation := range reply.Citations {
			if eval.Paragraph_Matches(citation.Paragraph, question.Paragraph) {
				pointHits++
				break
			}
		}
	}
	fmt.Printf(texts.EvalPointsLine, pointHits, pointQuestions)
	fmt.Printf(texts.EvalFallbackLine, fallbackCount, len(questions))
}

func login_Employee(client *http.Client, apiURL string) (string, error) {
	var session struct {
		Token string `json:"token"`
	}
	status, err := post_JSON(client, apiURL+"/auth/login", "", map[string]string{
		"login":    texts.SeedEmployeeLogin,
		"password": texts.SeedEmployeePassword,
	}, &session)
	if err != nil {
		return "", errors.New(texts.EvalCoreUnavailable)
	}
	if status != http.StatusOK || session.Token == "" {
		return "", errors.New(texts.EvalLoginFailed)
	}
	return session.Token, nil
}

func ask_Question(client *http.Client, apiURL, token, question string) (assistantLine, error) {
	var created struct {
		SessionID string `json:"sessionId"`
	}
	status, err := post_JSON(client, apiURL+"/sessions", token, map[string]string{}, &created)
	if err != nil {
		return assistantLine{}, errors.New(texts.EvalCoreUnavailable)
	}
	if status != http.StatusOK || created.SessionID == "" {
		return assistantLine{}, errors.New(texts.EvalRequestFailed)
	}
	var reply assistantLine
	status, err = post_JSON(client, apiURL+"/sessions/"+created.SessionID+"/messages", token, map[string]string{"text": question}, &reply)
	if err != nil {
		return assistantLine{}, errors.New(texts.EvalCoreUnavailable)
	}
	if status != http.StatusOK {
		return assistantLine{}, errors.New(texts.EvalRequestFailed)
	}
	return reply, nil
}

func post_JSON(client *http.Client, url, token string, body any, dst any) (int, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return 0, err
	}
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return 0, err
	}
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := client.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return response.StatusCode, err
	}
	if dst != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, dst); err != nil {
			return response.StatusCode, err
		}
	}
	return response.StatusCode, nil
}
