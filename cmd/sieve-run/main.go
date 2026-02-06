package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net/textproto"
	"os"
	"strings"
	"time"

	"github.com/foxcpp/go-sieve"
	"github.com/foxcpp/go-sieve/interp"
)

func main() {
	msgPath := flag.String("eml", "", "msgPath message to process")
	scriptPath := flag.String("scriptPath", "", "scriptPath to run")
	envFrom := flag.String("from", "", "envelope from")
	envTo := flag.String("to", "", "envelope to")
	flag.Parse()

	msg, err := os.Open(*msgPath)
	if err != nil {
		log.Fatalln(err)
	}
	defer msg.Close()
	fileInfo, err := msg.Stat()
	if err != nil {
		log.Fatalln(err)
	}
	msgHdr, err := textproto.NewReader(bufio.NewReader(msg)).ReadMIMEHeader()
	if err != nil {
		log.Fatalln(err)
	}

	script, err := os.Open(*scriptPath)
	if err != nil {
		log.Fatalln(err)
	}
	defer script.Close()

	start := time.Now()
	opts := sieve.DefaultOptions()
	// Enable all extensions
	opts.EnabledExtensions = []string{
		"fileinto", "envelope", "encoded-character",
		"comparator-i;octet", "comparator-i;ascii-casemap",
		"comparator-i;ascii-numeric", "comparator-i;unicode-casemap",
		"imap4flags", "variables", "relational", "vacation", "copy", "regex",
		"date", "index", "editheader", "mailbox", "subaddress",
	}
	loadedScript, err := sieve.Load(script, opts)
	end := time.Now()
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("script loaded in", end.Sub(start))

	envData := interp.EnvelopeStatic{
		From: *envFrom,
		To:   *envTo,
	}
	msgData := interp.MessageStatic{
		Size:   int(fileInfo.Size()),
		Header: msgHdr,
	}
	data := sieve.NewRuntimeData(loadedScript, interp.DummyPolicy{},
		envData, msgData)

	ctx := context.Background()
	start = time.Now()
	if err := loadedScript.Execute(ctx, data); err != nil {
		log.Fatalln(err)
	}
	end = time.Now()
	log.Println("script executed in", end.Sub(start))

	fmt.Println("redirect:", data.RedirectAddr)
	fmt.Println("fileinfo:", data.Mailboxes)
	fmt.Println("keep:", data.ImplicitKeep || data.Keep)
	fmt.Printf("flags: %s\n", strings.Join(data.Flags, " "))

	// Print vacation responses
	if len(data.VacationResponses) > 0 {
		fmt.Println("vacation responses:")
		for recipient, resp := range data.VacationResponses {
			fmt.Printf("  To: %s\n", recipient)
			fmt.Printf("  From: %s\n", resp.From)
			fmt.Printf("  Subject: %s\n", resp.Subject)
			fmt.Printf("  Body: %s\n", resp.Body)
			fmt.Printf("  Handle: %s\n", resp.Handle)
			fmt.Printf("  Days: %d\n", resp.Days)
			fmt.Printf("  MIME: %v\n", resp.IsMime)
			fmt.Println()
		}
	} else {
		fmt.Println("vacation responses: none")
	}
}
