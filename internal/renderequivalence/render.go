package renderequivalence

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
)

//go:embed render.rb
var renderScript string

// asciidoctor is one Ruby process that renders every document of a test run,
// because starting Asciidoctor takes far longer than a render.
var asciidoctor = sync.OnceValues(startAsciidoctor)

type renderer struct {
	mu  sync.Mutex
	in  *json.Encoder
	out *json.Decoder
}

func startAsciidoctor() (*renderer, error) {
	cmd := exec.Command("ruby", "-e", renderScript)
	cmd.Stderr = os.Stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting asciidoctor: %w", err)
	}
	return &renderer{in: json.NewEncoder(in), out: json.NewDecoder(out)}, nil
}

// render converts src to an HTML page with Asciidoctor.
//
// Warnings are dropped: malformed input is a legitimate test case, and the same
// warning appears on both sides of every comparison.
func render(src []byte) (string, error) {
	r, err := asciidoctor()
	if err != nil {
		return "", err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.in.Encode(string(src)); err != nil {
		return "", fmt.Errorf("asciidoctor: %w", err)
	}
	var reply struct {
		HTML  string `json:"html"`
		Error string `json:"error"`
	}
	if err := r.out.Decode(&reply); err != nil {
		return "", fmt.Errorf("asciidoctor: %w", err)
	}
	if reply.Error != "" {
		return "", errors.New("asciidoctor: " + reply.Error)
	}
	return reply.HTML, nil
}
