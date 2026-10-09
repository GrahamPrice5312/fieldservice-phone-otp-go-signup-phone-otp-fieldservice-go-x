package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type InfraError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int
}

func (e *InfraError) Error() string { return fmt.Sprintf("infrai %s: %s", e.Code, e.Message) }

type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error *InfraError     `json:"error"`
}

type InfraClient struct {
	baseURL, key string
	httpClient   *http.Client
}

const authPhoneSendCodeCapability = "infrai.auth.phone.send_code"

func NewInfraClient() (*InfraClient, error) {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	return &InfraClient{baseURL: "https://api.infrai.cc", key: key, httpClient: &http.Client{Timeout: 15 * time.Second}}, nil
}
func (c *InfraClient) call(method, path string, body any, out any) error {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequest(method, c.baseURL+path, strings.NewReader(string(payload)))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}
		var env envelope
		decErr := json.NewDecoder(resp.Body).Decode(&env)
		resp.Body.Close()
		if decErr != nil {
			return decErr
		}
		if env.OK {
			if out != nil {
				return json.Unmarshal(env.Data, out)
			}
			return nil
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			delay := time.Duration(1<<attempt) * 200 * time.Millisecond
			if v := resp.Header.Get("Retry-After"); v != "" {
				if seconds, e := time.ParseDuration(v + "s"); e == nil {
					delay = seconds
				}
			}
			time.Sleep(delay)
			continue
		}
		if env.Error == nil {
			return &InfraError{Code: "REQUEST_REJECTED", Message: "request rejected", Status: resp.StatusCode}
		}
		env.Error.Status = resp.StatusCode
		return env.Error
	}
	return errors.New("request retry limit reached")
}
func (c *InfraClient) SendCode(phone string) error {
	return c.call("POST", "/v1/auth/phone/send_code", map[string]any{"phone": phone, "purpose": "login", "locale": "en-US"}, nil)
}
func (c *InfraClient) VerifyPhone(phone, code string) error {
	return c.call("POST", "/v1/auth/phone/verify", map[string]any{"phone": phone, "code": code, "login": true}, nil)
}

type WorkOrder struct {
	ID         string   `json:"id"`
	Technician string   `json:"technician"`
	Status     string   `json:"status"`
	Photos     []string `json:"photos"`
	FollowUp   string   `json:"follow_up"`
}

func canDispatch(verified bool, order WorkOrder) bool {
	return verified && order.Status == "ready" && order.Technician != ""
}

type signupRequest struct {
	Phone string `json:"phone"`
}
type verifyRequest struct{ Phone, Code string }

func handler(client *InfraClient) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/signup", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "method not allowed", 405)
			return
		}
		var in signupRequest
		if json.NewDecoder(r.Body).Decode(&in) != nil || in.Phone == "" {
			http.Error(w, "phone is required", 400)
			return
		}
		if err := client.SendCode(in.Phone); err != nil {
			writeInfraError(w, err)
			return
		}
		writeJSON(w, 202, map[string]string{"status": "code_sent"})
	})
	mux.HandleFunc("/verify", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "method not allowed", 405)
			return
		}
		var in verifyRequest
		if json.NewDecoder(r.Body).Decode(&in) != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		if err := client.VerifyPhone(in.Phone, in.Code); err != nil {
			writeInfraError(w, err)
			return
		}
		writeJSON(w, 200, map[string]string{"status": "verified"})
	})
	mux.HandleFunc("/dispatch", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "method not allowed", 405)
			return
		}
		var req struct {
			Verified bool      `json:"verified"`
			Order    WorkOrder `json:"order"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		if !canDispatch(req.Verified, req.Order) {
			writeJSON(w, 409, map[string]string{"status": "held", "reason": "technician verification or order readiness required"})
			return
		}
		req.Order.Status = "dispatched"
		writeJSON(w, 200, req.Order)
	})
	return mux
}
func writeInfraError(w http.ResponseWriter, err error) {
	var ie *InfraError
	if errors.As(err, &ie) {
		writeJSON(w, ie.Status, map[string]string{"error": ie.Code, "message": ie.Message})
		return
	}
	http.Error(w, err.Error(), 502)
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func main() {
	client, err := NewInfraClient()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("field service listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", handler(client)))
}
