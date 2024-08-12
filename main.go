package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"sync"
	"syscall"
)

type SocketConnection struct {
	ID        string
	Conn      *websocket.Conn
	Mutex     *sync.Mutex
	IsClosing bool
	Files     []string
}

type Message struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type Postman struct {
	From    *websocket.Conn
	Message []byte
}

var Clients = make(map[*websocket.Conn]*SocketConnection)
var RegisterCh = make(chan *websocket.Conn)
var UnregisterCh = make(chan *websocket.Conn)
var PostmanCh = make(chan *Postman)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg := &sync.WaitGroup{}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	app := fiber.New()

	wg.Add(1)

	go func() {
		defer wg.Done()
		InitWorker(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := InitWebServer(app, wg)
		if err != nil {
			log.Fatal("error during starting web server", err)
		}
	}()

	<-stop
	log.Println("receiving exit signal")

	cancel()

	err := app.Shutdown()
	if err != nil {
		log.Println("error during shutting down webserver", err)
		return
	}

	wg.Wait()

	log.Println("shutdown complete, exiting")
}

func InitWebServer(app *fiber.App, wg *sync.WaitGroup) error {

	app.Use(func(c *fiber.Ctx) error {
		wg.Add(1)
		defer wg.Done()

		return c.Next()
	})

	app.Static("/", "./public")

	app.Use("/ws", func(c *fiber.Ctx) error {
		// IsWebSocketUpgrade returns true if the client
		// requested upgrade to the WebSocket protocol.
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get("/ws", websocket.New(func(c *websocket.Conn) {
		defer func() {
			UnregisterCh <- c
			err := c.Close()
			if err != nil {
				return
			}
		}()

		RegisterCh <- c

		for {
			_, m, err := c.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Println("read error:", err)
				}

				return // Calls the deferred function, i.e. closes the connection on error
			}

			PostmanCh <- &Postman{
				From:    c,
				Message: m,
			}
		}
	}))

	return app.Listen(":80")
}

func InitWorker(ctx context.Context) {
	wg := &sync.WaitGroup{}
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		case conn := <-RegisterCh:
			wg.Add(1)
			id := uuid.New().String()
			Clients[conn] = &SocketConnection{
				ID: id,
			}
			wg.Done()
		case conn := <-UnregisterCh:
			wg.Add(1)
			if _, ok := Clients[conn]; ok {
				cl := Clients[conn]
				cl.IsClosing = true
				err := removeFile(cl.Files)
				if err != nil {
					log.Println(err)
				}
				delete(Clients, conn)
			}
			wg.Done()
		case postman := <-PostmanCh:
			wg.Add(1)
			HandleMessage(postman)
			wg.Done()
		}
	}
}

func HandleMessage(p *Postman) {
	var msg Message
	cl := Clients[p.From]
	err := json.Unmarshal(p.Message, &msg)
	if err != nil {
		log.Println(err)
		err := sendMessage(p.From, "error", "error during parse json: "+err.Error())
		if err != nil {
			log.Println(err)
			return
		}
	}

	if msg.Type == "render" {
		url, fileName, err := renderPdf(msg.Message)
		if err != nil {
			log.Println(err)
			err = sendMessage(p.From, "error", "error during render: "+err.Error())
		}

		cl.Files = append(cl.Files, fileName)

		err = sendMessage(p.From, "rendered", url)
	} else {
		err := sendMessage(p.From, "error", msg.Message)
		if err != nil {
			log.Println(err)
		}
	}
}

func sendMessage(c *websocket.Conn, t, message string) error {
	client := Clients[c]

	if client.IsClosing {
		return errors.New("client is closing")
	}

	err := c.WriteJSON(map[string]interface{}{
		"type":    t,
		"message": message,
	})
	if err != nil {
		return err
	}
	return nil
}

func renderPdf(content string) (string, string, error) {

	f, err := saveToTemp(content)
	if err != nil {
		return "", "", err
	}
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			log.Println(err)
		}
	}(f.Name())

	filename, err := render(f)
	if err != nil {
		return "", "", err
	}

	return fmt.Sprintf("pdfjs/web/viewer.html?file=/storage/%s", filename), filename, nil
}

func saveToTemp(content string) (*os.File, error) {
	f, err := os.CreateTemp("public/storage", "html-rendered.*.htm")

	if err != nil {
		return nil, err
	}
	log.Println(f.Name())

	_, err = f.Write([]byte(content))

	if err != nil {
		return nil, err
	}

	return f, nil
}

func render(in *os.File) (string, error) {
	name := fmt.Sprintf("rendered-%s.pdf", uuid.New().String())
	outPath := path.Join("public/storage/", name)
	cmd := exec.Command("wkhtmltopdf", in.Name(), outPath)
	_, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return name, nil
}

func removeFile(files []string) error {
	for _, f := range files {
		err := os.Remove(fmt.Sprintf("public/storage/%s", f))
		if err != nil {
			return err
		}
	}

	return nil
}
