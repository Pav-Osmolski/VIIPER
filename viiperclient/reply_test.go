package viiperclient

import (
	"errors"
	"io"
	"strings"
	"testing"
)

var replyReset = errors.New("synthetic connection reset")

type fragmentReply struct {
	bytes []byte
	tail  error
	past  int
}

func (r *fragmentReply) Read(p []byte) (int, error) {
	if len(r.bytes) == 0 {
		r.past++
		return 0, r.tail
	}
	p[0] = r.bytes[0]
	r.bytes = r.bytes[1:]
	return 1, nil
}

func TestManagementReplyCompletion(t *testing.T) {
	for _, tc := range []struct {
		name, input, want string
		tail              error
		fails             bool
	}{
		{"reset after complete line", "{\"server\":\"é🎮\"}\n", "{\"server\":\"é🎮\"}", replyReset, false},
		{"legacy clean EOF", "{\"ok\":true}", "{\"ok\":true}", io.EOF, false},
		{"valid JSON without LF then reset", "{\"ok\":true}", "", replyReset, true},
		{"partial JSON then reset", "{\"ok\":", "", replyReset, true},
		{"empty reply", "\n", "", replyReset, false},
		{"only first line", "one\ntwo\n", "one", replyReset, false},
		{"pretty JSON then reset", "{\n\"ok\":true\n}\n", "{\n\"ok\":true\n}", replyReset, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := &fragmentReply{bytes: []byte(tc.input), tail: tc.tail}
			got, err := readManagementReply(reader)
			if tc.fails {
				if !errors.Is(err, replyReset) {
					t.Fatalf("wanted reset, got %q, %v", got, err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
			if strings.Contains(tc.input, "\n") && reader.past != 0 {
				t.Fatal("read after complete line")
			}
		})
	}
}
